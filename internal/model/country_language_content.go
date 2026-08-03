package model

// CountryLanguageContent 语言文本管理
type CountryLanguageContent struct {
	ID    int64  `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                               // ID
	Lan   string `gorm:"column:lan;type:varchar(20);not null;uniqueIndex:uk_country_language_content_lan_group_key,priority:1;comment:语言代码" json:"lan"`   // 语言代码
	Group string `gorm:"column:group;type:varchar(50);not null;uniqueIndex:uk_country_language_content_lan_group_key,priority:2;comment:分组" json:"group"` // 分组
	Key   string `gorm:"column:key;type:varchar(100);not null;uniqueIndex:uk_country_language_content_lan_group_key,priority:3;comment:键" json:"key"`     // 键
	Type  string `gorm:"column:type;type:varchar(30);not null;comment:类型:0=文本,1=富文本,2=图片" json:"type"`                                                    // 类型:0=文本,1=富文本,2=图片
	Value string `gorm:"column:value;type:longtext;comment:值" json:"value"`                                                                               // 值
}
