package crudgen

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/dbschema"

	"gorm.io/gorm"
)

// TableStore 是生成/物化阶段需要的表元数据读写原语。由
// *dbschema.Store 实现；crudgen 只依赖这个窄接口，不 import admin/repository。
type TableStore interface {
	Name(tableName string, fullName bool) string
	GetColumns(tableName string) ([]dbschema.Column, error)
	GetTableCollation(tableName string) (string, error)
	DelTable(tableName string) error
}

// NewTableStore 用当前连接与配置构造元数据读取器。
func NewTableStore(db *gorm.DB, cfg *conf.Configuration) *dbschema.Store {
	return dbschema.NewStore(db, cfg.Database.Database, cfg.Database.Prefix)
}
