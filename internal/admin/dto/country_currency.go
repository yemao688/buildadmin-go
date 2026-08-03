package dto

import "buildadmin-go/internal/pkg/validator"

type CurrencyParam struct {
	Code   string               `json:"code"`   // 货币代码
	Name   string               `json:"name"`   // 货币名称
	Symbol string               `json:"symbol"` // 货币符号
	Rate   validator.FlexFloat64 `json:"rate"`   // 汇率
	Status validator.FlexInt32   `json:"status"` // 状态:0=禁用,1=启用
	Weigh  validator.FlexInt32   `json:"weigh"`  // 权重
}
