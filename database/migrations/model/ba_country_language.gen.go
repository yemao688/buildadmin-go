package model

type CountryLanguage struct {
	ID     int64  `gorm:"column:id;type:bigint unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	Lan    string `gorm:"column:lan;type:varchar(20);not null;default:'';comment:语言代码" json:"lan"`
	Name   string `gorm:"column:name;type:varchar(50);not null;default:'';comment:语言名称" json:"name"`
	Remark string `gorm:"column:remark;type:varchar(255);not null;default:'';comment:备注" json:"remark"`
	Status int8   `gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`
	Weigh  int32  `gorm:"column:weigh;type:int;not null;default:0;comment:权重" json:"weigh"`
}
