package dto

import "buildadmin-go/internal/pkg/validator"

type CountryLanguageParam struct {
	Lan    string              `json:"lan"`    // 语言代码
	Name   string              `json:"name"`   // 语言名称
	Remark string              `json:"remark"` // 备注
	Status validator.FlexInt32 `json:"status"` // 状态:0=禁用,1=启用
	Weigh  validator.FlexInt32 `json:"weigh"`  // 权重
}
