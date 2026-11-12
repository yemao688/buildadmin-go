package dto

import (
	"encoding/json"
	"fmt"

	"buildadmin-go/internal/pkg/validator"
)

// CrudUploadCompletedParam 是 CRUD 上传完成回调参数。
type CrudUploadCompletedParam struct {
	SyncIDs    validator.FlexInt32Map `json:"syncIds"`
	CancelSync boolValue              `json:"cancelSync"`
}

// boolValue 兼容布尔与 0/1 数字两种 cancelSync 形态；保持非导出，调用方只
// 依赖布尔语义而不绑定具体类型。
type boolValue bool

func (b *boolValue) UnmarshalJSON(data []byte) error {
	var value bool
	if err := json.Unmarshal(data, &value); err == nil {
		*b = boolValue(value)
		return nil
	}

	var number int
	if err := json.Unmarshal(data, &number); err == nil && (number == 0 || number == 1) {
		*b = boolValue(number == 1)
		return nil
	}

	return fmt.Errorf("cancelSync must be a boolean")
}
