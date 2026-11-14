// Package business 是业务仓库扩展迁移轨道的注册与执行入口。
package business

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"buildadmin-go/internal/conf"
	"gorm.io/gorm"
)

type Migration struct {
	Version       uint64
	MigrationName string
	// NeedsSpecTables 列出 Up 之前必须按 crud_specs 物化的业务表（逻辑表名）。
	// runner 在 Up 前逐张调用 EnsureSpecTable；只建 schema/索引，不建菜单。
	// rename/DROP 列的迁移不要声明，否则目标形状会先被建出来。
	NeedsSpecTables   []string
	Up                func(*gorm.DB, *conf.Configuration) error
	Down              func(*gorm.DB, *conf.Configuration) error
	VerifyBaseline    func(*gorm.DB, *conf.Configuration) error
	VerifySchema      func(*gorm.DB, *conf.Configuration) error
	VerifyUpgradeData func(*gorm.DB, *conf.Configuration) error
}

var (
	mu       sync.Mutex
	registry []Migration
	frozen   bool
)

func Register(m Migration) {
	mu.Lock()
	defer mu.Unlock()
	if frozen {
		panic("business migration registry is frozen")
	}
	registry = append(registry, m)
}

func Migrations() ([]Migration, error) {
	mu.Lock()
	defer mu.Unlock()
	frozen = true
	list := append([]Migration(nil), registry...)
	sort.Slice(list, func(i, j int) bool { return list[i].Version < list[j].Version })
	if err := validateMigrations(list); err != nil {
		return nil, err
	}
	return list, nil
}

func validateMigrations(list []Migration) error {
	seen := map[string]bool{}
	for i, migration := range list {
		if migration.Version != uint64(i+1) || strings.TrimSpace(migration.MigrationName) == "" || seen[migration.MigrationName] || migration.Up == nil {
			return fmt.Errorf("invalid business migration at index %d", i)
		}
		seen[migration.MigrationName] = true
		specTables := map[string]bool{}
		for _, table := range migration.NeedsSpecTables {
			name := strings.TrimSpace(table)
			if name == "" {
				return fmt.Errorf("business migration %q declares an empty spec table name", migration.MigrationName)
			}
			if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
				return fmt.Errorf("business migration %q declares invalid spec table %q: spec file name must equal the logical table name", migration.MigrationName, table)
			}
			if specTables[name] {
				return fmt.Errorf("business migration %q declares spec table %q more than once", migration.MigrationName, name)
			}
			specTables[name] = true
		}
	}
	return nil
}
