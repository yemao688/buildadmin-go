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
	Sequence     uint64
	ID           string
	Revision     uint64
	Up           func(*gorm.DB, *conf.Configuration) error
	VerifySchema func(*gorm.DB, *conf.Configuration) error
	VerifyData   func(*gorm.DB, *conf.Configuration) error
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
	sort.Slice(list, func(i, j int) bool { return list[i].Sequence < list[j].Sequence })
	seen := map[string]bool{}
	for i, migration := range list {
		if migration.Sequence != uint64(i+1) || strings.TrimSpace(migration.ID) == "" || seen[migration.ID] || migration.Revision == 0 || migration.Up == nil {
			return nil, fmt.Errorf("invalid business migration at index %d", i)
		}
		seen[migration.ID] = true
	}
	return list, nil
}
