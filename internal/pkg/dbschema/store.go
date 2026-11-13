package dbschema

import (
	"errors"
	"strings"

	"buildadmin-go/internal/pkg/data_scope"

	"gorm.io/gorm"
)

// Store 是生成器需要的表元数据读写原语。它只依赖 *gorm.DB 与前缀，
// 不依赖任何业务渠道，因此 crudgen 可以直接使用而不反向 import admin。
type Store struct {
	db       *gorm.DB
	database string
	prefix   string
}

// NewStore 构造元数据读取器。database 为空时使用当前连接的默认库。
func NewStore(db *gorm.DB, database, prefix string) *Store {
	return &Store{db: db, database: database, prefix: prefix}
}

func (s *Store) schemaClause() (string, []any) {
	if s.database == "" {
		return "DATABASE()", nil
	}
	return "?", []any{s.database}
}

// Name 返回带/不带前缀的表名，与 TableRepository 的命名规则一致。
func (s *Store) Name(tableName string, fullName bool) string {
	prefix := ""
	if fullName {
		prefix = s.prefix
	}
	tableName = strings.TrimPrefix(tableName, s.prefix)
	return prefix + tableName
}

func (s *Store) GetColumns(tableName string) ([]Column, error) {
	result := []Column{}
	schemaExpr, schemaArgs := s.schemaClause()
	args := append(schemaArgs, s.Name(tableName, true))
	err := s.db.Raw("SELECT * FROM `information_schema`.`columns` WHERE TABLE_SCHEMA = "+schemaExpr+" AND table_name = ? ORDER BY ORDINAL_POSITION", args...).Scan(&result).Error
	return result, err
}

func (s *Store) GetTableCollation(tableName string) (string, error) {
	var collation string
	schemaExpr, schemaArgs := s.schemaClause()
	args := append(schemaArgs, s.Name(tableName, true))
	err := s.db.Raw("SELECT TABLE_COLLATION FROM information_schema.tables WHERE TABLE_SCHEMA = "+schemaExpr+" AND table_name = ? LIMIT 1", args...).Scan(&collation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return collation, err
}

func (s *Store) DelTable(tableName string) error {
	tableName = s.Name(tableName, true)
	if err := data_scope.ValidateIdentifier(tableName); err != nil {
		return err
	}
	return s.db.Exec("DROP TABLE IF EXISTS `" + tableName + "`").Error
}
