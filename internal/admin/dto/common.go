// Package dto 存放后台渠道的请求参数（一表一个 XxxParam，Add/Edit 复用），只做绑定与校验声明。
package dto

import "buildadmin-go/internal/pkg/validator"

// IDS 是通用 ID 参数结构，用于 Edit/Delete 等需要单 ID 的接口。
type IDS struct {
	ID validator.FlexInt32 `json:"id" binding:"required"`
}
