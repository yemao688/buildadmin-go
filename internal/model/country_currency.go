package model

// CountryCurrency 全局货币
type CountryCurrency struct {
	ID     int64   `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement:true;comment:ID" json:"id"`                              // ID
	Code   string  `gorm:"column:code;type:varchar(20);not null;uniqueIndex:uk_country_currency_code,priority:1;comment:货币代码" json:"code"` // 货币代码
	Name   string  `gorm:"column:name;type:varchar(50);not null;comment:货币名称" json:"name"`                                                 // 货币名称
	Symbol string  `gorm:"column:symbol;type:varchar(20);not null;comment:货币符号" json:"symbol"`                                             // 货币符号
	Rate   float64 `gorm:"column:rate;type:decimal(20,8);not null;default:1.00000000;comment:汇率" json:"rate"`                              // 汇率
	Status int32   `gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`                      // 状态:0=禁用,1=启用
	Weigh  int32   `gorm:"column:weigh;type:int;not null;comment:权重" json:"weigh"`                                                         // 权重
}
