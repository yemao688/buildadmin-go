package model

type CountryCurrency struct {
	ID     int64   `gorm:"column:id;type:bigint unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	Code   string  `gorm:"column:code;type:varchar(20);not null;default:'';comment:货币代码" json:"code"`
	Name   string  `gorm:"column:name;type:varchar(50);not null;default:'';comment:货币名称" json:"name"`
	Symbol string  `gorm:"column:symbol;type:varchar(20);not null;default:'';comment:货币符号" json:"symbol"`
	Rate   float64 `gorm:"column:rate;type:decimal(20,8);not null;default:1;comment:汇率" json:"rate"`
	Status int8    `gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`
	Weigh  int32   `gorm:"column:weigh;type:int;not null;default:0;comment:权重" json:"weigh"`
}
