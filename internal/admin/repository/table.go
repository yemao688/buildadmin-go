package repository

import (
	"database/sql"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	"slices"
	"strings"

	"gorm.io/gorm"
)

type TableRepository struct {
	config *conf.Configuration
	sqlDB  *gorm.DB
}

func NewTableRepository(config *conf.Configuration, sqlDB *gorm.DB) *TableRepository {
	return &TableRepository{
		sqlDB:  sqlDB,
		config: config,
	}
}

func (s *TableRepository) DB() *gorm.DB {
	return s.sqlDB
}

// schemaClause 返回 information_schema 查询的 schema 定位表达式与参数：
// 配置了 Database 时参数化取值；为空时回退 DATABASE()（当前连接默认库，
// 与 GetTablePk/IsExist 的既有模式一致）。否则空字符串作为参数会静默匹配
// 0 行且无错误（information_schema 不报 schema 不存在），导致 GetColumns
// 返回空列被 primaryKeyDrift 误判（如 EnsureSpecTable 二次物化时
// cfg.Database 仅含 Prefix 的场景）。
func (s *TableRepository) schemaClause() (string, []any) {
	if s.config.Database.Database == "" {
		return "DATABASE()", nil
	}
	return "?", []any{s.config.Database.Database}
}

// 获取数据表的名称,包含数据表前缀
func (s *TableRepository) Name(tableName string, fullName bool) string {
	prefix := ""
	if fullName {
		prefix = s.config.Database.Prefix
	}
	tableName = strings.TrimPrefix(tableName, s.config.Database.Prefix)
	return prefix + tableName
}

// 获取数据库的所有数据表
func (s *TableRepository) GetTableList() map[string]string {
	type Table struct {
		TABLE_NAME    string
		TABLE_COMMENT string
	}
	var tableList []Table
	schemaExpr, schemaArgs := s.schemaClause()
	s.sqlDB.Raw("SELECT TABLE_NAME,TABLE_COMMENT FROM information_schema.TABLES WHERE table_schema = "+schemaExpr, schemaArgs...).Scan(&tableList)
	data := map[string]string{}
	for _, v := range tableList {
		if v.TABLE_COMMENT != "" {
			data[v.TABLE_NAME] = v.TABLE_NAME + " - " + v.TABLE_COMMENT
		} else {
			data[v.TABLE_NAME] = v.TABLE_NAME
		}
	}
	return data
}

type TableListItem struct {
	Table      string `json:"table"`
	Comment    string `json:"comment"`
	Connection string `json:"connection"`
	Prefix     string `json:"prefix"`
}

// 获取数据表列表（v2.3.7 前端格式）
// 对齐上游 Ajax::getTableList：samePrefix 默认仅返回同前缀表，table 值去除
// 表前缀（前端据此组合关联字段名，如 admin_id），excludeTable 为无前缀表名。
func (s *TableRepository) GetTableListV2(quickSearch string, samePrefix bool, excludeTable []string) []TableListItem {
	type Table struct {
		TABLE_NAME    string
		TABLE_COMMENT string
	}
	var tableList []Table
	schemaExpr, schemaArgs := s.schemaClause()
	query := "SELECT TABLE_NAME,TABLE_COMMENT FROM information_schema.TABLES WHERE table_schema = " + schemaExpr

	result := []TableListItem{}
	if err := s.sqlDB.Raw(query, schemaArgs...).Scan(&tableList).Error; err != nil {
		return result
	}
	prefix := s.config.Database.Prefix
	for _, v := range tableList {
		if quickSearch != "" && !strings.Contains(v.TABLE_NAME, quickSearch) && !strings.Contains(v.TABLE_COMMENT, quickSearch) {
			continue
		}
		name := StripTablePrefix(v.TABLE_NAME, prefix)
		if samePrefix && name == v.TABLE_NAME {
			continue
		}
		if slices.Contains(excludeTable, name) {
			continue
		}
		item := TableListItem{
			Table:      name,
			Comment:    v.TABLE_NAME + " - " + v.TABLE_COMMENT,
			Connection: "mysql",
			Prefix:     prefix,
		}
		if v.TABLE_COMMENT == "" {
			item.Comment = v.TABLE_NAME
		}
		result = append(result, item)
	}
	return result
}

// StripTablePrefix 去除表名前缀（大小写不敏感，对齐上游 preg_replace('/^prefix/i')）。
func StripTablePrefix(tableName, prefix string) string {
	if prefix != "" && len(tableName) >= len(prefix) && strings.EqualFold(tableName[:len(prefix)], prefix) {
		return tableName[len(prefix):]
	}
	return tableName
}

// 获取表主键字段
func (s *TableRepository) GetTablePk(tableName string) string {
	if tableName == "" {
		return ""
	}
	tableName = s.Name(tableName, true)

	var columnName string
	s.sqlDB.Raw("SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = 'PRIMARY'", tableName).Scan(&columnName)
	return columnName
}

type Column struct {
	COLUMN_NAME           string
	COLUMN_COMMENT        string
	IS_NULLABLE           string
	COLUMN_TYPE           string
	DATA_TYPE             string
	COLUMN_DEFAULT        sql.NullString
	COLUMN_KEY            string
	EXTRA                 string
	CHARACTER_SET_NAME    string
	COLLATION_NAME        string
	GENERATION_EXPRESSION string
}

// 获取数据表的所有字段
func (s *TableRepository) GetTableFields(tableName string, onlyCleanComment bool) map[string]any {
	if tableName == "" {
		return nil
	}
	tableName = s.Name(tableName, true)

	var columnList []Column
	schemaExpr, schemaArgs := s.schemaClause()
	args := append(schemaArgs, tableName)
	s.sqlDB.Raw("SELECT * FROM `information_schema`.`columns` WHERE TABLE_SCHEMA = "+schemaExpr+" AND table_name = ? ORDER BY ORDINAL_POSITION", args...).Scan(&columnList)
	data := map[string]any{}
	for _, v := range columnList {
		if onlyCleanComment {
			data[v.COLUMN_NAME] = ""
			if v.COLUMN_COMMENT != "" {
				comment := strings.Split(v.COLUMN_COMMENT, ":")
				data[v.COLUMN_NAME] = comment[0]
			}
			continue
		}
		data[v.COLUMN_NAME] = v
	}
	return data
}

// 获取表信息
func (s *TableRepository) GetInfo(tableName string) ([]map[string]any, error) {
	result := []map[string]any{}
	schemaExpr, schemaArgs := s.schemaClause()
	args := append(schemaArgs, s.Name(tableName, true))
	err := s.sqlDB.Raw("SELECT * FROM `information_schema`.`tables` WHERE TABLE_SCHEMA = "+schemaExpr+" AND table_name = ?", args...).Scan(&result).Error
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *TableRepository) GetColumns(tableName string) ([]Column, error) {
	result := []Column{}
	schemaExpr, schemaArgs := s.schemaClause()
	args := append(schemaArgs, s.Name(tableName, true))
	err := s.sqlDB.Raw("SELECT * FROM `information_schema`.`columns`  WHERE TABLE_SCHEMA = "+schemaExpr+" AND table_name = ? ORDER BY ORDINAL_POSITION", args...).Scan(&result).Error
	if err != nil {
		return result, err
	}
	return result, nil
}

// 数据表是否有数据
func (s *TableRepository) IsHasData(tableName string) (bool, error) {
	result := []map[string]any{}
	err := s.sqlDB.Raw("select * from `?` LIMIT 1", tableName).Scan(&result).Error
	if err != nil {
		return false, err
	}

	if len(result) == 0 {
		return false, nil
	}
	return true, nil
}

// 修改数据表字段备注
func (s *TableRepository) ChangeComment(tableName string, comment string) error {
	err := s.sqlDB.Exec("ALTER TABLE `?` COMMENT = `?`", tableName, comment).Error
	return err
}

// 删除数据表
func (s *TableRepository) DelTable(tableName string) error {
	tableName = s.Name(tableName, true)
	if err := data_scope.ValidateIdentifier(tableName); err != nil {
		return err
	}
	err := s.sqlDB.Exec("DROP TABLE IF EXISTS `" + tableName + "`").Error
	return err
}

// 数据表是否存在
func (s *TableRepository) IsExist(tableName string) bool {
	result := map[string]int{}
	s.sqlDB.Raw("SELECT COUNT(*) AS num FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", tableName).Scan(&result)
	return result["num"] == 1
}
