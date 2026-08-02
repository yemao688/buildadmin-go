package dto

import shared "go-build-admin/internal/model"

// OutUser 前台会员对外展示投影（User 实体统一在 internal/model 共享记录层）
type OutUser struct {
	shared.User
	Money string `json:"money"`
}
