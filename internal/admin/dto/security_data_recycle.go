package dto

import "buildadmin-go/internal/pkg/validator"

// SecurityDataRecycleParam 是回收站规则（表 security_data_recycle）新增/编辑
// 请求的绑定 DTO。controller_as 由 service 从 controller 归一化，请求体中的
// controller_as 不参与写入。
type SecurityDataRecycleParam struct {
	Name         string               `json:"name"`
	Controller   string               `json:"controller"`
	ControllerAs string               `json:"controller_as"`
	DataTable    string               `json:"data_table"`
	PrimaryKey   string               `json:"primary_key"`
	Status       validator.FlexStatus `json:"status"`
}

func (v SecurityDataRecycleParam) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{}
}
