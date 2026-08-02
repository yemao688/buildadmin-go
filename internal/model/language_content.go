package model

import "gorm.io/gorm/schema"

// TableName 经全局命名策略解析（前缀安全），对齐真实表 country_language_content。
func (LanguageContent) TableName(namer schema.Namer) string {
	return namer.TableName("country_language_content")
}

// LanguageContent 语言文本管理
type LanguageContent struct {
	ID    int64  `gorm:"column:id;type:bigint unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                      // ID
	Lan   string `gorm:"column:lan;type:varchar(20);not null;default:'';uniqueIndex:uk_country_language_content_lan_group_key;comment:语言代码" json:"lan"`   // 语言代码
	Group string `gorm:"column:group;type:varchar(50);not null;default:'';uniqueIndex:uk_country_language_content_lan_group_key;comment:分组" json:"group"` // 分组
	Key   string `gorm:"column:key;type:varchar(100);not null;default:'';uniqueIndex:uk_country_language_content_lan_group_key;comment:键" json:"key"`     // 键
	Type  string `gorm:"column:type;type:varchar(30);not null;default:'';comment:类型:0=文本,1=富文本,2=图片" json:"type"`                                         // 类型:0=文本,1=富文本,2=图片
	Value string `gorm:"column:value;type:longtext;comment:值" json:"value"`                                                                               // 值
}
