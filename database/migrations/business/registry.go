package business

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

type Migration struct {
	Version           uint64
	MigrationName     string
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
	seen := map[string]bool{}
	for i, migration := range list {
		if migration.Version != uint64(i+1) || strings.TrimSpace(migration.MigrationName) == "" || seen[migration.MigrationName] || migration.Up == nil {
			return nil, fmt.Errorf("invalid business migration at index %d", i)
		}
		seen[migration.MigrationName] = true
	}
	return list, nil
}
