package model

// LanguageContent 语言文本管理
type LanguageContent struct {
	ID    int64  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"` // ID
	Lan   string `gorm:"column:lan;not null;comment:语言代码" json:"lan"`                  // 语言代码
	Group string `gorm:"column:group;not null;comment:分组" json:"group"`                // 分组
	Key   string `gorm:"column:key;not null;comment:键" json:"key"`                     // 键
	Type  string `gorm:"column:type;not null;comment:类型:0=文本,1=富文本,2=图片" json:"type"`  // 类型:0=文本,1=富文本,2=图片
	Value string `gorm:"column:value;comment:值" json:"value"`                          // 值
}
