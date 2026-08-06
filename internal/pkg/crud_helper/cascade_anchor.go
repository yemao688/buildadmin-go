package crud_helper

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"buildadmin-go/internal/conf"
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"gorm.io/gorm"
)

// 级联注册锚点：reassignable 主实体 repo 生成的 CascadeOwners() 方法体内
// 以这两行注释为界，生成器在子表生成/删除时自动维护其中的注册条目。锚点
// 标记各独占一行，中间初始为空。
const (
	cascadeAnchorBegin = "// @cascade:begin"
	cascadeAnchorEnd   = "// @cascade:end"
)

// cascadeAnchorEntry 生成锚点块内的一行注册条目。固定格式，OwnerColumn 恒为
// admin_id（级联硬契约：归属列必须为 admin_id）。
func cascadeAnchorEntry(table, byColumn string) string {
	return "\t\t{Table: \"" + table + "\", ByColumn: \"" + byColumn + "\", OwnerColumn: \"admin_id\"},"
}

// parentRepositoryPath 推导主实体 repo 文件路径（拍平布局
// internal/admin/repository/<table>.go），经既有路径解析与绝对路径归属校验
// 防注入路径。
func parentRepositoryPath(table string) (string, error) {
	info, err := ParseRepositoryNameData(table, "")
	if err != nil {
		return "", err
	}
	if err := ValidateGeneratedAbsolutePath(info.ParseFile, "internal/admin/repository"); err != nil {
		return "", err
	}
	return info.ParseFile, nil
}

// findCascadeAnchor 定位锚点起止行号。begin/end 均为 -1 表示缺失；ok 仅在
// 两者都存在且 begin 在 end 之前时为真。
func findCascadeAnchor(content string) (begin, end int, ok bool) {
	begin, end = -1, -1
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == cascadeAnchorBegin {
			begin = i
		}
		if trimmed == cascadeAnchorEnd {
			end = i
		}
	}
	return begin, end, begin >= 0 && end >= 0 && begin < end
}

// validateInheritParent 校验 inheritFrom 指向的主实体已成功生成且声明了
// reassignable（其 repo 才有 CascadeOwners() 锚点块可注册），并落实级联硬
// 契约：主实体主键必须为 id、归属列必须为 admin_id（模板与 cascade:sync 均
// 按 `Where("id = ?")`/`Select("admin_id")`/JOIN `p.id` 拼接 SQL）。抽成独立
// 函数便于 MySQL 门禁单测；GenerateFromSpec 在子表生成前调用。
func validateInheritParent(db *gorm.DB, cfg *conf.Configuration, parentTable string) error {
	log, err := latestSuccessfulCrudLog(db, cfg, parentTable)
	if err != nil {
		return err
	}
	if log == nil {
		return fmt.Errorf("inheritFrom parent %q has no successful CRUD generation record; generate the parent table with dataScope.reassignable=true first", parentTable)
	}
	if log.Table.DataScope == nil || !log.Table.DataScope.Reassignable {
		return fmt.Errorf("inheritFrom parent %q was not generated with dataScope.reassignable=true; regenerate the parent table first", parentTable)
	}
	// 硬契约：主实体归属列必须是 admin_id（auto 模式 ownerColumn 为空，视为
	// 解析为 admin_id；required 显式声明时必须是 admin_id）。
	if owner := log.Table.DataScope.OwnerColumn; owner != "" && owner != "admin_id" {
		return fmt.Errorf("inheritFrom parent %q declares owner column %q; cascade contract requires admin_id", parentTable, owner)
	}
	// 硬契约：主实体主键必须为 id（模板 `Where("id = ?", ...)` 与
	// cascade:sync 的 JOIN `p.id` 按此拼接；非 id 主键运行时必炸）。
	pks := specPrimaryKeys([]crudmodel.Field(log.Fields))
	if len(pks) != 1 || pks[0] != "id" {
		return fmt.Errorf("inheritFrom parent %q primary key must be exactly id (cascade contract), got %v", parentTable, pks)
	}
	return nil
}

// reapplyCascadeAnchors 在主实体重新生成后调用：重新生成会把 CascadeOwners()
// 渲染为空锚点块（重置窗口），此处按 crud_log 中仍声明 inheritFrom 指向该主
// 实体的子表逐条重注册（applyCascadeAnchor 整行幂等），消除重置窗口内的静默
// 级联失效。无引用时为空操作。
func reapplyCascadeAnchors(db *gorm.DB, cfg *conf.Configuration, parentTable string) error {
	referrers, err := findInheritReferrers(db, cfg, parentTable)
	if err != nil {
		return err
	}
	if len(referrers) == 0 {
		return nil
	}
	repoPath, err := parentRepositoryPath(parentTable)
	if err != nil {
		return err
	}
	for _, child := range referrers {
		childLog, err := latestSuccessfulCrudLog(db, cfg, child)
		if err != nil {
			return err
		}
		if childLog == nil || childLog.Table.DataScope == nil || childLog.Table.DataScope.InheritFrom == nil {
			continue
		}
		if err := applyCascadeAnchor(repoPath, child, childLog.Table.DataScope.InheritFrom.ByColumn); err != nil {
			return fmt.Errorf("reapply cascade anchor for child %q: %w", child, err)
		}
	}
	return nil
}

// inheritRefOf 从 data-scope 配置中取出 inheritFrom 声明（nil 安全）。
func inheritRefOf(cfg *data_scope.Config) *data_scope.InheritRef {
	if cfg == nil {
		return nil
	}
	return cfg.InheritFrom
}

// staleInheritParent 比较子表旧生成记录与新 spec 的 inheritFrom 指向，返回
// 需要清理锚点条目的旧主实体表名：旧记录声明过 inheritFrom 且新声明为 nil
// 或指向不同表时返回旧表名；指向未变（含 byColumn 变化）返回空串（由
// applyCascadeAnchor 的整行替换处理）。
func staleInheritParent(oldLog *crudmodel.Log, newInherit *data_scope.InheritRef) string {
	if oldLog == nil || oldLog.Table.DataScope == nil || oldLog.Table.DataScope.InheritFrom == nil {
		return ""
	}
	oldParent := oldLog.Table.DataScope.InheritFrom.Table
	if newInherit != nil && newInherit.Table == oldParent {
		return ""
	}
	return oldParent
}

// latestSuccessfulCrudLogs 读取 crud_log 全部行并按 table_name 去重：每个表只
// 保留最新一行（create_time desc, id desc），该行 status 不是 success 则跳过该
// 表——与 CLI cascade:sync 的 loadCrudLogRows 语义一致。
// 必须"先按最新行去重、再过滤 success"：crud:delete 只把最新一条 success 行原
// 地翻转为 delete（updateCrudStatus 按 log.ID），若先按 status 过滤，历史遗留
// 的旧 success 行会把已删除模块复活为悬空引用（评审 blocker，回归见
// TestFindInheritReferrersMySQL 的 delete 消费场景）。
func latestSuccessfulCrudLogs(db *gorm.DB, cfg *conf.Configuration) ([]crudmodel.Log, error) {
	var scanned []struct {
		TableName string               `gorm:"column:table_name"`
		Table     crudmodel.JSON_TABLE `gorm:"column:table"`
		Status    string               `gorm:"column:status"`
	}
	if err := db.Table(crudLogTable(cfg)).
		Select("table_name", "`table`", "status").
		Order("create_time desc, id desc").
		Scan(&scanned).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(scanned))
	logs := make([]crudmodel.Log, 0, len(scanned))
	for _, s := range scanned {
		if _, ok := seen[s.TableName]; ok {
			continue
		}
		seen[s.TableName] = struct{}{}
		if s.Status != "success" {
			continue
		}
		logs = append(logs, crudmodel.Log{Tablename: s.TableName, Table: s.Table})
	}
	return logs, nil
}

// findInheritReferrers 扫描 crud_log 最新 success 记录，返回 inheritFrom 指向
// parentTable 的子表名列表（确定性排序）。主实体自身的记录不计入引用。
func findInheritReferrers(db *gorm.DB, cfg *conf.Configuration, parentTable string) ([]string, error) {
	logs, err := latestSuccessfulCrudLogs(db, cfg)
	if err != nil {
		return nil, err
	}
	var referrers []string
	for _, log := range logs {
		if log.Tablename == parentTable {
			continue
		}
		if log.Table.DataScope != nil && log.Table.DataScope.InheritFrom != nil &&
			log.Table.DataScope.InheritFrom.Table == parentTable {
			referrers = append(referrers, log.Tablename)
		}
	}
	sort.Strings(referrers)
	return referrers, nil
}

// cascadeAnchorEntryPattern 匹配锚点块内指向指定子表的一行注册条目。空白容忍
// （gofmt/手改的空格差异不破坏幂等与替换判定）；表名经 regexp.QuoteMeta 转义，
// 表名尾部引号限定整词（child 不误伤 child2）。
func cascadeAnchorEntryPattern(table string) *regexp.Regexp {
	return regexp.MustCompile(`\{\s*Table\s*:\s*"` + regexp.QuoteMeta(table) + `"\s*,`)
}

// applyCascadeAnchor 向主实体 repo 的锚点块写入一条子表注册条目。严格要求：
// 文件必须存在且含完整锚点块，否则报错（子表生成前主实体必须先以
// reassignable 生成）。幂等与替换按整行匹配（空白容忍）：
//   - 锚点块内已存在完全相同的整行 → 跳过写入（幂等）；
//   - 存在同 Table 的整行（含 ByColumn 不同或手改过格式的）→ 原位替换为新
//     条目，且同 Table 的重复条目全部收敛为一条（子表重新生成改 byColumn 时
//     不残留旧列，避免级联按旧列 UPDATE 静默漂移；手改残留的重复行也会收敛）；
//   - 否则在 `// @cascade:end` 之前插入。
func applyCascadeAnchor(repoFilePath, table, byColumn string) error {
	content, err := os.ReadFile(repoFilePath)
	if err != nil {
		return fmt.Errorf("apply cascade anchor: read parent repository %q: %w", repoFilePath, err)
	}
	begin, end, ok := findCascadeAnchor(string(content))
	if !ok {
		return fmt.Errorf("apply cascade anchor: anchor block not found in %q; regenerate the parent table with a reassignable dataScope first", repoFilePath)
	}
	entry := cascadeAnchorEntry(table, byColumn)
	pattern := cascadeAnchorEntryPattern(table)
	lines := strings.Split(string(content), "\n")
	// 单遍扫描：锚点块内同 Table 的行收敛为一条规范条目，其余行原样保留。
	// 精确条目存在时也继续扫描（收敛手改残留的重复行），仅在内容变化时写回。
	// 未命中任何同 Table 行时，在 `// @cascade:end`（下标 end）之前插入。
	kept := make([]string, 0, len(lines)+1)
	seeded := false // 是否已在块内放置规范条目
	changed := false
	for i, line := range lines {
		if i == end && !seeded {
			kept = append(kept, entry)
			seeded = true
			changed = true
		}
		if i <= begin || i >= end {
			kept = append(kept, line)
			continue
		}
		if line == entry {
			if !seeded {
				kept = append(kept, line)
				seeded = true
			} else {
				changed = true // 重复的精确条目：收敛
			}
			continue
		}
		if pattern.MatchString(line) {
			if !seeded {
				kept = append(kept, entry)
				seeded = true
			}
			changed = true // 变体/旧 ByColumn 行：替换或收敛
			continue
		}
		kept = append(kept, line)
	}
	if !changed {
		return nil
	}
	return writePreservingMode(repoFilePath, strings.Join(kept, "\n"))
}

// removeCascadeAnchor 从主实体 repo 的锚点块移除子表注册条目。幂等容忍：
// 文件不存在或锚点缺失直接返回 nil（防止 crud:delete 因主实体已删/锚点
// 结构漂移而失败）；条目不存在同样直接返回 nil。
func removeCascadeAnchor(repoFilePath, table string) error {
	content, err := os.ReadFile(repoFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	begin, end, ok := findCascadeAnchor(string(content))
	if !ok {
		return nil
	}
	pattern := cascadeAnchorEntryPattern(table)
	lines := strings.Split(string(content), "\n")
	updated := make([]string, 0, len(lines))
	removed := false
	for i, line := range lines {
		if i > begin && i < end && pattern.MatchString(line) {
			removed = true
			continue
		}
		updated = append(updated, line)
	}
	if !removed {
		return nil
	}
	return writePreservingMode(repoFilePath, strings.Join(updated, "\n"))
}

// writePreservingMode 原子写回：同目录临时文件 + rename，崩溃不留下截断的
// 主实体 repo（评审 MINOR：os.WriteFile 原地截断，中断会破坏文件并使后续
// 生成失败）。
func writePreservingMode(path, content string) error {
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
