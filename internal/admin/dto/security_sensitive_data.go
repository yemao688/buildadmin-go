package dto

import "buildadmin-go/internal/pkg/validator"

// SecuritySensitiveDataParam 是敏感数据规则（表 security_sensitive_data）新增/
// 编辑请求的绑定 DTO。controller_as 由 service 从 controller 归一化，请求体中
// 的 controller_as 不参与写入。
type SecuritySensitiveDataParam struct {
	Name         string               `json:"name"`
	Controller   string               `json:"controller"`
	ControllerAs string               `json:"controller_as"`
	DataTable    string               `json:"data_table"`
	PrimaryKey   string               `json:"primary_key"`
	Fields       []SensitiveField     `json:"fields"`
	Status       validator.FlexStatus `json:"status"`
}

func (v SecuritySensitiveDataParam) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{}
}

// SensitiveField 是敏感数据规则中一个字段的展示名/取值对。
type SensitiveField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
