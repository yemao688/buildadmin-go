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
}

// PreviewGenerationFiles 计算生成的产物清单，不写任何文件也不连数据库。
// 与 crud:apply --plan 组合构成 crud:generate --dry-run 的完整预览。
func PreviewGenerationFiles(_ *gorm.DB, _ *conf.Configuration, table crudmodel.Table, fields []crudmodel.Field) (GenerationFilePlan, error) {
	manifest, err := BuildFileManifestForFields(table, fields)
	if err != nil {
		return GenerationFilePlan{}, err
	}
	plan := GenerationFilePlan{}
	for _, path := range append(append([]string{}, manifest.Generated...), manifest.Shared...) {
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
