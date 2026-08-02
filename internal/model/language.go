package model

import "gorm.io/gorm/schema"

// TableName 经全局命名策略解析（前缀安全），对齐真实表 country_language。
func (Language) TableName(namer schema.Namer) string {
	return namer.TableName("country_language")
}

// Language 全局语言
type Language struct {
	ID     int64  `gorm:"column:id;type:bigint unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`                  // 主键
	Lan    string `gorm:"column:lan;type:varchar(20);not null;default:'';uniqueIndex:uk_country_language_lan;comment:语言代码" json:"lan"` // 语言代码
	Name   string `gorm:"column:name;type:varchar(50);not null;default:'';comment:语言名称" json:"name"`                                   // 语言名称
	Remark string `gorm:"column:remark;type:varchar(255);not null;default:'';comment:备注" json:"remark"`                                // 备注
	Status int32  `gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`                   // 状态:0=禁用,1=启用
	Weigh  int32  `gorm:"column:weigh;type:int;not null;default:0;comment:权重" json:"weigh"`                                            // 权重
}
