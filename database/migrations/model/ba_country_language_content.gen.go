package model

type CountryLanguageContent struct {
	ID    int64  `gorm:"column:id;type:bigint unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	Lan   string `gorm:"column:lan;type:varchar(20);not null;default:'';comment:语言代码" json:"lan"`
	Group string `gorm:"column:group;type:varchar(50);not null;default:'';comment:分组" json:"group"`
	Key   string `gorm:"column:key;type:varchar(100);not null;default:'';comment:键" json:"key"`
	Type  string `gorm:"column:type;type:varchar(30);not null;default:'';comment:类型" json:"type"`
	Value string `gorm:"column:value;type:longtext;comment:值" json:"value"`
}
