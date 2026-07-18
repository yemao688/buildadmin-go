package model

import (
	"gorm.io/gorm"
)

type BaseModel struct {
	TableName        string
	Key              string
	QuickSearchField string
	sqlDB            *gorm.DB
}

func (s *BaseModel) DB() *gorm.DB {
	return s.sqlDB
}

func (s *BaseModel) Table() string {
	return s.TableName
}

func (s *BaseModel) PrimaryKeyName() string {
	return s.Key
}

func (s *BaseModel) TableInfo() TableInfo {
	return TableInfo{
		TableName:        s.TableName,
		Key:              s.Key,
		QuickSearchField: s.QuickSearchField,
	}
}
