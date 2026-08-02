package dto

import "buildadmin-go/internal/admin/validate"

type LanguageParam struct {
	Lan    string             `json:"lan"`    // 语言代码
	Name   string             `json:"name"`   // 语言名称
	Remark string             `json:"remark"` // 备注
	Status validate.FlexInt32 `json:"status"` // 状态:0=禁用,1=启用
	Weigh  validate.FlexInt32 `json:"weigh"`  // 权重
}
