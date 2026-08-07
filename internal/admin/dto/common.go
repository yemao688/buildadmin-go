package dto

import "buildadmin-go/internal/pkg/validator"

// IDS 是通用 ID 参数结构，用于 Edit/Delete 等需要单 ID 的接口。
type IDS struct {
	ID validator.FlexInt32 `json:"id" binding:"required"`
}
