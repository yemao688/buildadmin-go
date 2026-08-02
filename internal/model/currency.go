package model

import "gorm.io/gorm/schema"

// TableName 经全局命名策略解析（前缀安全），对齐真实表 country_currency。
func (Currency) TableName(namer schema.Namer) string {
	return namer.TableName("country_currency")
}

// Currency 全局货币
type Currency struct {
	ID     int64   `gorm:"column:id;type:bigint unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`                     // 主键
	Code   string  `gorm:"column:code;type:varchar(20);not null;default:'';uniqueIndex:uk_country_currency_code;comment:货币代码" json:"code"` // 货币代码
	Name   string  `gorm:"column:name;type:varchar(50);not null;default:'';comment:货币名称" json:"name"`                                      // 货币名称
	Symbol string  `gorm:"column:symbol;type:varchar(20);not null;default:'';comment:货币符号" json:"symbol"`                                  // 货币符号
	Rate   float64 `gorm:"column:rate;type:decimal(20,8);not null;default:1.00000000;comment:汇率" json:"rate"`                              // 汇率
	Status int32   `gorm:"column:status;type:tinyint unsigned;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"`                      // 状态:0=禁用,1=启用
	Weigh  int32   `gorm:"column:weigh;type:int;not null;default:0;comment:权重" json:"weigh"`                                               // 权重
}
