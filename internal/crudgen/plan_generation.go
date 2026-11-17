package crudgen

import (
	"buildadmin-go/internal/conf"
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"sort"

	"gorm.io/gorm"
)

// GenerationFilePlan 描述一次生成会触碰的文件：Create 是不存在、将被新建的
// 产物；Overwrite 是已存在、将被覆盖的产物（含共享 provider/wire 文件）。
type GenerationFilePlan struct {
	Create    []string
	Overwrite []string
	// NewTable 表示该表自己的产物一个都还不存在（共享 provider/wire 不算）。
	// 此时无法渲染预览：实体文件由 gorm.io/gen 从真实表结构生成，表还没建。
	NewTable bool
	// TableMissing 表示目标库里还没有这张表：在线实体渲染依赖真实表结构
	// （information_schema），无法渲染预览。产物可能已存在（全新库、
	// crud.apply_on_migrate=false、表被手工删除），与 NewTable 相互独立。
	TableMissing bool
}

// PreviewGenerationFiles 计算生成的产物清单，不写任何文件；只读地检查表是否
// 已存在，用于判断预览能否渲染。
// 与 crud:apply --plan 组合构成 crud:generate --dry-run 的完整预览。
func PreviewGenerationFiles(db *gorm.DB, cfg *conf.Configuration, table crudmodel.Table, fields []crudmodel.Field) (GenerationFilePlan, error) {
	manifest, err := BuildFileManifestForFields(table, fields)
	if err != nil {
		return GenerationFilePlan{}, err
	}
	plan := GenerationFilePlan{NewTable: true}
	if db != nil && cfg != nil {
		plan.TableMissing = !tableExists(db, cfg, table.Name)
	}
	for index, path := range append(append([]string{}, manifest.Generated...), manifest.Shared...) {
		if index < len(manifest.Generated) && fileExists(path) {
			plan.NewTable = false
		}
		if fileExists(path) {
			plan.Overwrite = append(plan.Overwrite, path)
			continue
		}
		plan.Create = append(plan.Create, path)
	}
	sort.Strings(plan.Create)
	sort.Strings(plan.Overwrite)
	return plan, nil
}
