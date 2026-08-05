package business

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"buildadmin-go/internal/conf"
	helper "buildadmin-go/internal/pkg/crud_helper"

	"gorm.io/gorm"
)

// EnsureSpecTable 在业务迁移 Up 内物化一张由 crud_specs 声明的业务表。
//
// 背景：全新库编排为「业务迁移（Up）→ 尾部 apply」，而业务表 schema 的唯一
// 物化路径是 spec → crud:apply。种子迁移需要在 apply 之前拿到表（写种子），
// 但不能用 AutoMigrate/裸 SQL 建表（双重事实源、绕过安全矩阵）。本 helper
// 是唯一允许的替代：在 Up 内调用它，按 spec 幂等物化该表（全新库 ApplyCreated
// 无阻塞；已有库按安全矩阵处理，requires-approval/rejected 会报错阻止迁移，
// 种子不能建在错误形状上）。
//
// 只物化 schema 与索引，不建菜单——菜单由 migrate/apply 尾部的全量 apply
// 统一同步。表的所有权归 spec/apply：本迁移的 Down 不得 DROP 依赖表。
func EnsureSpecTable(db *gorm.DB, config *conf.Configuration, logicalName string) error {
	return EnsureSpecTableFrom(db, config, helper.DefaultSpecDir(), logicalName)
}

// EnsureSpecTableFrom 与 EnsureSpecTable 相同，但显式指定 spec 目录
// （测试与嵌入场景使用；DefaultSpecDir 为空的场景直接报错）。
func EnsureSpecTableFrom(db *gorm.DB, config *conf.Configuration, dir, logicalName string) error {
	if dir == "" {
		return fmt.Errorf("EnsureSpecTable(%q): crud_specs directory not found", logicalName)
	}
	specPath := filepath.Join(dir, logicalName+".yaml")
	if _, err := os.Stat(specPath); err != nil {
		return fmt.Errorf("EnsureSpecTable(%q): spec %s not found (spec filename must equal the logical table name)", logicalName, specPath)
	}
	results, err := helper.ApplySpecs(db, config, []string{specPath}, helper.ApplyOptions{AdminID: 1, SkipMenu: true})
	if err != nil {
		var blocked *helper.ApplyBlockedError
		if errors.As(err, &blocked) {
			return fmt.Errorf("EnsureSpecTable(%q) blocked: %s", logicalName, blocked.Hint())
		}
		return fmt.Errorf("EnsureSpecTable(%q): %w", logicalName, err)
	}
	if len(results) == 0 {
		return fmt.Errorf("EnsureSpecTable(%q): apply returned no result", logicalName)
	}
	switch results[0].Action {
	case helper.ApplyCreated, helper.ApplyUnchanged, helper.ApplyAltered:
		return nil
	default:
		return fmt.Errorf("EnsureSpecTable(%q): unexpected apply action %q", logicalName, results[0].Action)
	}
}
