package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// crud:apply —— 部署语义的 spec 应用。
//
// 与开发态 crud:generate 的分工：generate 负责"代码 + 开发库 DDL + 菜单"，
// apply 负责把仓库里提交的 spec 幂等地同步到任意目标库（表结构 alter、
// 菜单按 name 去重、crud_log adopt），不写代码、不产生迁移文件。
//
// 安全门（部署工具与开发工具的本质区别）：
//   - 表不存在：按 spec 初始建表（这不是"重建"，安全）。
//   - 表已存在：永远只走 alter 安全子集；spec 的 type 是生成时动词，不是稳态
//     重建指令——type:create 的规格在已有表上同样幂等（无漂移即 unchanged）。
//   - 主键漂移（alter 物理不支持的重建场景）：拒绝并指向 business 迁移；
//     仅限可丢弃环境可显式 AllowRebuild 删除重建（数据丢失）。

type ApplyAction string

const (
	ApplyCreated   ApplyAction = "created"
	ApplyAltered   ApplyAction = "altered"
	ApplyUnchanged ApplyAction = "unchanged"
	ApplyRebuilt   ApplyAction = "rebuilt"
)

type ApplyOptions struct {
	AllowRebuild bool // 允许对已有表按 spec type:create 删除重建（数据丢失，仅限可丢弃环境）
	SkipMenu     bool
	AdminID      int32
}

type ApplyTableResult struct {
	Table   string
	Action  ApplyAction
	Changes []string
	LogID   int32
}

// decideApplyAction 计算已有表的应用动作与拒绝原因（纯函数，便于测试）。
// 只有主键漂移才会触发重建判定；其余差量一律走 alter。
func decideApplyAction(pkDrift bool, allowRebuild bool, tableName, actualPK, specPK string) (ApplyAction, error) {
	if !pkDrift {
		return ApplyAltered, nil
	}
	if !allowRebuild {
		return "", fmt.Errorf("primary key drift on %q: database=%q spec=%q; apply only supports alter-safe changes — hand-write a business migration for primary key changes with backfill, or use --allow-rebuild on disposable environments", tableName, actualPK, specPK)
	}
	return ApplyRebuilt, nil
}

// ApplySpecsFromDir 应用目录下全部 *.yaml spec（文件名排序保证确定性）；目录不存在或为空时静默跳过。
func ApplySpecsFromDir(db *gorm.DB, cfg *conf.Configuration, dir string, opts ApplyOptions) ([]ApplyTableResult, error) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("scan spec dir %q: %w", dir, err)
	}
	if len(entries) == 0 {
		return nil, nil
	}
	sort.Strings(entries)
	return ApplySpecs(db, cfg, entries, opts)
}

// ApplySpecs 逐个应用 spec；任一失败即中止（DDL 本就不可回滚，确定性顺序使重跑可续）。
func ApplySpecs(db *gorm.DB, cfg *conf.Configuration, specPaths []string, opts ApplyOptions) ([]ApplyTableResult, error) {
	if db == nil || cfg == nil {
		return nil, fmt.Errorf("crud apply requires database and configuration")
	}
	release, err := TryAcquireGenerationLock()
	if err != nil {
		return nil, err
	}
	defer release()
	// 生成锁已持有：任何仍为 start 的记录都是进程中断的残留，对账为失败
	reconcileStaleGeneratingLogs(db, cfg)

	if opts.AdminID <= 0 {
		opts.AdminID = 1
	}
	tableM := model.NewTableModel(cfg, db)
	results := make([]ApplyTableResult, 0, len(specPaths))
	for _, path := range specPaths {
		result, err := applyOneSpec(db, cfg, tableM, path, opts)
		if err != nil {
			return results, fmt.Errorf("apply %q: %w", path, err)
		}
		results = append(results, *result)
	}
	return results, nil
}

func applyOneSpec(db *gorm.DB, cfg *conf.Configuration, tableM *model.TableModel, specPath string, opts ApplyOptions) (*ApplyTableResult, error) {
	spec, err := LoadSpec(specPath)
	if err != nil {
		return nil, err
	}
	if IsProtectedTable(spec.Table.Name) {
		return nil, fmt.Errorf("crud apply is forbidden for protected table %q", spec.Table.Name)
	}
	genType := normalizeGenerationType(spec.Type, spec.Table.Rebuild)
	if err := validateGenerationMode(genType); err != nil {
		return nil, err
	}
	exists := tableExists(db, cfg, spec.Table.Name)
	fullName := tableM.Name(spec.Table.Name, true)
	result := &ApplyTableResult{Table: spec.Table.Name, Action: ApplyCreated}

	if exists {
		actualPK, err := actualPrimaryKey(db, fullName)
		if err != nil {
			return nil, fmt.Errorf("read primary key: %w", err)
		}
		specPK := getPk(spec.Fields)
		action, err := decideApplyAction(!strings.EqualFold(actualPK, specPK), opts.AllowRebuild, spec.Table.Name, actualPK, specPK)
		if err != nil {
			return nil, err
		}
		result.Action = action
		if action == ApplyRebuilt {
			if err := tableM.DelTable(spec.Table.Name); err != nil {
				return nil, fmt.Errorf("drop table: %w", err)
			}
		} else {
			current, err := tableM.GetColumns(spec.Table.Name)
			if err != nil {
				return nil, fmt.Errorf("read existing columns: %w", err)
			}
			spec.Table.DesignChange = deriveAlterChanges(current, spec.Fields)
			for _, change := range spec.Table.DesignChange {
				result.Changes = append(result.Changes, change.Type+" "+change.NewName)
			}
			if len(spec.Table.DesignChange) == 0 {
				result.Action = ApplyUnchanged
			}
		}
	}
	if err := HandleTableDesign(db, fullName, spec.Table, spec.Fields); err != nil {
		return nil, fmt.Errorf("table design: %w", err)
	}
	if !opts.SkipMenu {
		webViewsDir := ParseWebDirNameData(spec.Table.Name, "views", spec.Table.WebViewsDir)
		if _, err := CreateMenuWithOptionsAndRecord(model.NewAdminRuleModel(db, cfg), webViewsDir, spec.Table.Comment, spec.Menu); err != nil {
			return nil, fmt.Errorf("menu sync: %w", err)
		}
	}
	logID, err := adoptCrudLog(db, cfg, spec, opts.AdminID)
	if err != nil {
		return nil, fmt.Errorf("crud log adopt: %w", err)
	}
	result.LogID = logID
	return result, nil
}

// adoptCrudLog 确保目标库存在与当前 spec 一致的 success 记录：无则新建，
// 有则回写最新 spec payload（manifest 随当前布局重算，保持 crud:delete 可用性）。
func adoptCrudLog(db *gorm.DB, cfg *conf.Configuration, spec *GenerateOptions, adminID int32) (int32, error) {
	manifest, err := BuildFileManifestForFields(model.Table(spec.Table), []model.Field(spec.Fields))
	if err != nil {
		return 0, err
	}
	spec.Table.GeneratedFiles = append([]string(nil), append(append([]string{}, manifest.Generated...), manifest.Shared...)...)
	spec.Table.Manifest = &model.CRUDFileManifest{Generated: append([]string{}, manifest.Generated...), Shared: append([]string{}, manifest.Shared...)}
	spec.AdminID = adminID

	existing, err := latestSuccessfulCrudLog(db, cfg, spec.Table.Name)
	if err != nil {
		return 0, err
	}
	if existing == nil {
		logID, err := createCrudLog(db, cfg, *spec)
		if err != nil {
			return 0, err
		}
		if err := updateCrudStatus(db, cfg, logID, "success"); err != nil {
			return 0, err
		}
		return logID, nil
	}
	updates := map[string]interface{}{
		"table":  model.JSON_TABLE(spec.Table),
		"fields": model.JSON_FIELDS(spec.Fields),
	}
	if err := db.Table(crudLogTable(cfg)).Where("id=?", existing.ID).Updates(updates).Error; err != nil {
		return 0, err
	}
	return existing.ID, nil
}

// DefaultSpecDir 返回仓库内约定的 spec 目录（不存在时返回空串，供调用方静默跳过）。
func DefaultSpecDir() string {
	dir := filepath.Join(utils.RootPath(), "crud_specs")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}
