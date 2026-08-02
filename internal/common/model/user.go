package model

import shared "go-build-admin/internal/model"

// OutUser 前台会员对外展示投影（User 实体已统一迁移至 internal/model）
type OutUser struct {
	shared.User
	Money string `json:"money"`
}
