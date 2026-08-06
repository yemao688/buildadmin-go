package commands

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/infra/db"
	"buildadmin-go/internal/pkg/data_scope"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// cascadeOwnerColumn 是继承契约固定的归属列：子表冗余 admin_id 对账到主表
// 的 admin_id，join 用主表主键 id（文档硬契约：主表 PK=id）。
const cascadeOwnerColumn = "admin_id"

// newCascadeSyncCommand 构造 cascade:sync 对账命令：业务表冗余 admin_id
// 用于数据权限/统计，主实体改归属时生成器在事务内级联同步子表；本命令做
// 离线兜底对账。job 来源是子表 spec 的 dataScope.inheritFrom 反向聚合——
// 扫描 crud_log 中所有最新成功的子表记录，把子表归属列修正为其继承主表的
// 当前值。可选 [table] 参数限定只同步 inheritFrom 指向该主表的子表。
func newCascadeSyncCommand(cmdBootstrap CmdBootstrap) *cobra.Command {
	return &cobra.Command{
		Use:           "cascade:sync [table]",
		Short:         "按 crud_log 中子表 inheritFrom 声明对账归属列（离线修复脏数据）",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			command, cleanup, err := cmdBootstrap(config, loggerWriter, logger)
			if err != nil {
				return err
			}
			defer cleanup()
			return command.cascadeH.Sync(cmd, args)
		},
	}
}

// CascadeHandler 提供 cascade:sync 的运行时依赖，与 MigrateHandler/CrudHandler
// 同构：wire 注入 logger/config，自建 gorm 连接。
type CascadeHandler struct {
	logger *zap.Logger
	db     *gorm.DB
	config *conf.Configuration
}

func NewCascadeHandler(logger *zap.Logger, config *conf.Configuration) *CascadeHandler {
	return &CascadeHandler{
		logger: logger,
		db:     db.NewDB(config, logger),
		config: config,
	}
}

// LogRow 是 crud_log 行的最小结构：表名 + table 列 JSON 原文。
type LogRow struct {
	TableName string
	TableJSON string
}

// ChildRef 是子表侧的继承声明：子表名 + 关联主表 id 的列。
// 归属列固定 cascadeOwnerColumn（admin_id），不由 spec 携带。
type ChildRef struct {
	ChildTable string // 子表（冗余 admin_id 待对账）
	ByColumn   string // 子表关联主表 id 的列
}

// CascadeJob 描述一个主表及其全部 inheritFrom 子表的级联同步任务：
// 由 crud_log 中子表记录的 dataScope.inheritFrom 反向聚合而来。
type CascadeJob struct {
	ParentTable string     // 主实体表（admin_id 的事实源，PK=id）
	Children    []ChildRef // 声明 inheritFrom 指向主表的子表
}

// cascadeTableJSON 是 crud_log.table 列 JSON 的最小子集。字段名按生成器
// 约定（dataScope/inheritFrom/table/byColumn/name），刻意不依赖
// internal/pkg/data_scope.Config 的具体 Go 类型（另一会话正在重构生成器）。
// encoding/json 的字段匹配大小写不敏感，天然容错 key 大小写差异。
// 注意：主实体侧的旧 cascadeOwners 声明已移除，历史记录中的该字段被忽略。
type cascadeTableJSON struct {
	Name      string `json:"name"`
	DataScope *struct {
		InheritFrom *struct {
			Table    string `json:"table"`
			ByColumn string `json:"byColumn"`
		} `json:"inheritFrom"`
	} `json:"dataScope"`
}

// buildCascadeSyncJobs 从 crud_log 行反向聚合级联同步任务。决策说明：
//   - 过滤规则：只有 dataScope.inheritFrom 非空的行产出任务（该行即子表，
//     parsed.Name 是子表名，inheritFrom.table 是主表名）；
//   - 聚合：同一主表的所有子表归入一个 CascadeJob（如 user ← user_money_log、
//     order_recharge 两个子表 → 一个 user job 含两个 children），按行出现
//     顺序输出，确定性稳定；
//   - inheritFrom 缺 table 或 byColumn 视为不完整声明，直接报错（失败关闭）；
//   - table JSON 无法解析的行直接报错（失败关闭：对账工具宁可中断也不静默
//     漏掉潜在脏数据），错误信息携带 table_name 便于定位修复；
//   - 任何标识符（表名/列名）不合法即报错，绝不进入 SQL。
func buildCascadeSyncJobs(logs []LogRow) ([]CascadeJob, error) {
	jobs := make([]CascadeJob, 0)
	byParent := make(map[string]int) // parent table -> index in jobs
	for _, row := range logs {
		var parsed cascadeTableJSON
		if err := json.Unmarshal([]byte(row.TableJSON), &parsed); err != nil {
			return nil, fmt.Errorf("cascade:sync: crud_log table JSON for table %q is invalid: %w", row.TableName, err)
		}
		if parsed.DataScope == nil || parsed.DataScope.InheritFrom == nil {
			continue
		}
		inherited := parsed.DataScope.InheritFrom
		if inherited.Table == "" || inherited.ByColumn == "" {
			return nil, fmt.Errorf("cascade:sync: table %q declares incomplete inheritFrom (table=%q byColumn=%q)", row.TableName, inherited.Table, inherited.ByColumn)
		}
		child := ChildRef{ChildTable: parsed.Name, ByColumn: inherited.ByColumn}
		if err := validateChildRef(inherited.Table, child); err != nil {
			return nil, err
		}
		idx, ok := byParent[inherited.Table]
		if !ok {
			idx = len(jobs)
			byParent[inherited.Table] = idx
			jobs = append(jobs, CascadeJob{ParentTable: inherited.Table})
		}
		jobs[idx].Children = append(jobs[idx].Children, child)
	}
	return jobs, nil
}

// validateChildRef 校验主表名与单个子表引用的每个标识符（字母数字下划线，
// 拒绝点号/空格/引号等可解释为 SQL 语法的字符），非法即报错退出。
func validateChildRef(parentTable string, child ChildRef) error {
	for _, id := range []struct{ kind, value string }{
		{"parent table", parentTable},
		{"child table", child.ChildTable},
		{"by column", child.ByColumn},
		{"owner column", cascadeOwnerColumn},
	} {
		if err := data_scope.ValidateIdentifier(id.value); err != nil {
			return fmt.Errorf("cascade:sync: invalid %s %q in cascade job (parent=%q child=%q via %q owner=%q): %w",
				id.kind, id.value, parentTable, child.ChildTable, child.ByColumn, cascadeOwnerColumn, err)
		}
	}
	return nil
}

// validateCascadeJob 校验 job 内全部标识符（主表 + 每个子表引用），
// 与 runCascadeJob 共用同一 fail-closed 入口。
func validateCascadeJob(job CascadeJob) error {
	for _, child := range job.Children {
		if err := validateChildRef(job.ParentTable, child); err != nil {
			return err
		}
	}
	return nil
}

// runCascadeJob 对单个"主表 + 子表引用"执行级联同步，返回 RowsAffected：
//
//	UPDATE {prefix}{child} c JOIN {prefix}{parent} p ON c.{byColumn} = p.id
//	SET c.admin_id = p.admin_id WHERE c.admin_id <> p.admin_id
//
// 幂等：只动不一致行。表名 = 配置前缀 + crud_log JSON 静态标识符，执行前对
// 全部标识符（含拼接后的完整表名）做合法性校验，非法直接报错，绝不拼进 SQL。
// 执行前还检查父表存在性：主实体被 crud:delete/手工删表后遗留的悬空
// inheritFrom 声明若直接走 UPDATE JOIN 会裸报 1146 且不可读，这里转为
// 语义化错误（fail-closed，与整体错误传播策略一致）。子表自身存在性不预检
// （UPDATE 本身会报错，且父表缺失才是常见悬空场景）。
// 注意：NULL 归属列的行不满足 <> 比较（NULL <> x 为 NULL），保持原样，与
// 规格给出的 SQL 语义一致。
func runCascadeJob(db *gorm.DB, cfg *conf.Configuration, parentTable string, child ChildRef) (int64, error) {
	if err := validateChildRef(parentTable, child); err != nil {
		return 0, err
	}
	fullChild := cfg.Database.Prefix + child.ChildTable
	fullParent := cfg.Database.Prefix + parentTable
	for _, id := range []struct{ kind, value string }{
		{"child table", fullChild},
		{"parent table", fullParent},
	} {
		if err := data_scope.ValidateIdentifier(id.value); err != nil {
			return 0, fmt.Errorf("cascade:sync: invalid %s %q (config prefix + spec identifier): %w", id.kind, id.value, err)
		}
	}
	var exists int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", fullParent).Scan(&exists).Error; err != nil {
		return 0, fmt.Errorf("cascade:sync: check parent table %s existence: %w", fullParent, err)
	}
	if exists == 0 {
		return 0, fmt.Errorf("cascade:sync: parent table %q missing: child %q declares inheritFrom targeting it (recreate the parent or remove the child's inheritFrom declaration)", fullParent, fullChild)
	}
	stmt := fmt.Sprintf(
		"UPDATE `%s` c JOIN `%s` p ON c.`%s` = p.`id` SET c.`%s` = p.`%s` WHERE c.`%s` <> p.`%s`",
		fullChild, fullParent, child.ByColumn, cascadeOwnerColumn, cascadeOwnerColumn, cascadeOwnerColumn, cascadeOwnerColumn,
	)
	result := db.Exec(stmt)
	return result.RowsAffected, result.Error
}

// loadCrudLogRows 读取 crud_log 全部行的最新状态并按键去重：每个 table_name
// 只取最新一行（create_time desc, id desc），该行 status 不是 success 则跳过
// 该表——与 latestSuccessfulCrudLog（internal/pkg/crud_helper/generator.go）的
// delete 消费语义对齐：成功A→成功B→delete(B) 后，A 仍是 success 陈旧行，但
// 模块 A 已删除（子表可能已 DROP），按旧声明产出 job 会在 UPDATE 时报 1146
// 并使全量对账整体失败。只有最新一行确认为 success 的表才参与对账。
func loadCrudLogRows(db *gorm.DB, cfg *conf.Configuration) ([]LogRow, error) {
	var scanned []struct {
		TableName string `gorm:"column:table_name"`
		Table     string `gorm:"column:table"`
		Status    string `gorm:"column:status"`
	}
	if err := db.Table(cfg.Database.Prefix+"crud_log").
		// table 是 MySQL 保留字，Select 不会自动加反引号，这里显式引用。
		Select("table_name", "`table`", "status").
		Order("create_time desc, id desc").
		Scan(&scanned).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(scanned))
	rows := make([]LogRow, 0, len(scanned))
	for _, s := range scanned {
		if _, ok := seen[s.TableName]; ok {
			continue
		}
		seen[s.TableName] = struct{}{}
		if s.Status != "success" {
			continue
		}
		rows = append(rows, LogRow{TableName: s.TableName, TableJSON: s.Table})
	}
	return rows, nil
}

// Sync 执行级联对账主流程：
//   - 无参数：扫描全部最新成功的子表记录，按 inheritFrom 聚合后逐主表同步；
//   - 指定 table：只同步 inheritFrom 指向该主表的子表；无任何子表声明指向
//     它时明确报错退出（非 0）。
//
// 运行前先打印将处理的表清单（parent → child via byColumn，每子表 1 行），
// 每子表输出 {prefix}{child}: {n} rows reconciled，末尾汇总
// reconciled N rows across M tables（M 为实际执行的子表数）。
// 无待同步任务时输出 0 行并正常退出 0。
func (h *CascadeHandler) Sync(cmd *cobra.Command, args []string) error {
	rows, err := loadCrudLogRows(h.db, h.config)
	if err != nil {
		return fmt.Errorf("cascade:sync: load crud_log: %w", err)
	}
	jobs, err := buildCascadeSyncJobs(rows)
	if err != nil {
		return err
	}

	if len(args) == 1 {
		target := args[0]
		filtered := make([]CascadeJob, 0, len(jobs))
		for _, job := range jobs {
			if job.ParentTable == target {
				filtered = append(filtered, job)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("cascade:sync: no successful crud_log record declares inheritFrom targeting table %q", target)
		}
		jobs = filtered
	}

	for _, job := range jobs {
		for _, child := range job.Children {
			cmd.Printf("%s%s → %s%s via %s\n", h.config.Database.Prefix, job.ParentTable, h.config.Database.Prefix, child.ChildTable, child.ByColumn)
		}
	}

	var total int64
	var tables int
	for _, job := range jobs {
		for _, child := range job.Children {
			affected, err := runCascadeJob(h.db, h.config, job.ParentTable, child)
			if err != nil {
				cmd.Printf("%s%s: sync error: %v\n", h.config.Database.Prefix, child.ChildTable, err)
				return err
			}
			total += affected
			tables++
			cmd.Printf("%s%s: %d rows reconciled\n", h.config.Database.Prefix, child.ChildTable, affected)
		}
	}
	cmd.Printf("reconciled %d rows across %d tables\n", total, tables)
	return nil
}
