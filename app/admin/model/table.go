package model

import (
	"go-build-admin/conf"
	"strings"

	"gorm.io/gorm"
)

type TableModel struct {
	config *conf.Configuration
	sqlDB  *gorm.DB
}

func NewTableModel(config *conf.Configuration, sqlDB *gorm.DB) *TableModel {
	return &TableModel{
		sqlDB:  sqlDB,
		config: config,
	}
}

// 获取数据表的名称,包含数据表前缀
func (s *TableModel) Name(tableName string, fullName bool) string {
	prefix := ""
	if fullName {
		prefix = s.config.Database.Prefix
	}
	tableName = strings.TrimLeft(s.config.Database.Prefix, tableName)
	return prefix + tableName
}

// 获取数据库的所有数据表
func (s *TableModel) GetTableList() map[string]string {
	type Table struct {
		TABLE_NAME    string
		TABLE_COMMENT string
	}
	var tableList []Table
	s.sqlDB.Raw("SELECT TABLE_NAME,TABLE_COMMENT FROM information_schema.TABLES WHERE table_schema = ? ", s.config.Database.Database).Scan(&tableList)
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

// 获取表主键字段
func (s *TableModel) GetTablePk(tableName string) string {
	if tableName == "" {
		return ""
	}

	var columnName string
	s.sqlDB.Raw("SELECT COLUMN_NAME FROM INFORMATION_SCHEMA.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = 'PRIMARY'", tableName).Scan(&columnName)
	return columnName
}

// 获取数据表的所有字段
func (s *TableModel) GetTableFields(tableName string, onlyCleanComment bool) map[string]any {
	if tableName == "" {
		return nil
	}

	type Column struct {
		COLUMN_NAME    string
		COLUMN_COMMENT string
		IS_NULLABLE    string
		COLUMN_TYPE    string
		DATA_TYPE      string
		COLUMN_DEFAULT string
		COLUMN_KEY     string
		EXTRA          string
	}
	var columnList []Column
	s.sqlDB.Raw("SELECT * FROM `information_schema`.`columns` WHERE TABLE_SCHEMA = ? AND table_name = ? ORDER BY ORDINAL_POSITION", s.config.Database.Database, tableName).Scan(&columnList)
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
func (s *TableModel) GetInfo(tableName string) ([]map[string]string, error) {
	result := []map[string]string{}
	err := s.sqlDB.Raw("SELECT * FROM `information_schema`.`tables` WHERE TABLE_SCHEMA = ? AND table_name = ?", s.config.Database.Database, tableName).Scan(&result).Error
	if err != nil {
		return result, err
	}

	if len(result) == 0 {
		return result, nil
	}
	return result, nil
}

// 数据表是否有数据
func (s *TableModel) IsHasData(tableName string) (bool, error) {
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
func (s *TableModel) ChangeComment(tableName string, comment string) error {
	err := s.sqlDB.Exec("ALTER TABLE `?` COMMENT = `?`", tableName, comment).Error
	return err
}

// 删除数据表
func (s *TableModel) DelTable(tableName string) error {
	tableName = s.Name(tableName, true)
	err := s.sqlDB.Exec("DROP TABLE IF EXISTS `?`", tableName).Error
	return err
}

// 数据表是否存在
func (s *TableModel) IsExist(tableName string) bool {
	result := map[string]int{}
	s.sqlDB.Raw("SELECT COUNT(*) AS num FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ", tableName).Scan(&result)
	return result["num"] == 1
}
