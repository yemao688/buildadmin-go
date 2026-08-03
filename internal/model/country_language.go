package model

// CountryLanguage 全局语言
type CountryLanguage struct {
	ID     int64  `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement:true;comment:ID" json:"id"`                           // ID
	Lan    string `gorm:"column:lan;type:varchar(20);not null;uniqueIndex:uk_country_language_lan,priority:1;comment:语言代码" json:"lan"` // 语言代码
	Name   string `gorm:"column:name;type:varchar(50);not null;comment:语言名称" json:"name"`                                              // 语言名称
	Remark string `gorm:"column:remark;type:varchar(255);not null;comment:备注" json:"remark"`                                           // 备注
	Status int32  `gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`                   // 状态:0=禁用,1=启用
	Weigh  int32  `gorm:"column:weigh;type:int;not null;comment:权重" json:"weigh"`                                                      // 权重
}
