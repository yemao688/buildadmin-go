package model

// Currency 全局货币
type Currency struct {
	ID     int64   `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // 主键
	Code   string  `gorm:"column:code;not null;comment:货币代码" json:"code"`                       // 货币代码
	Name   string  `gorm:"column:name;not null;comment:货币名称" json:"name"`                       // 货币名称
	Symbol string  `gorm:"column:symbol;not null;comment:货币符号" json:"symbol"`                   // 货币符号
	Rate   float64 `gorm:"column:rate;not null;default:1.00000000;comment:汇率" json:"rate"`      // 汇率
	Status int32   `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	Weigh  int32   `gorm:"column:weigh;not null;comment:权重" json:"weigh"`                       // 权重
}
