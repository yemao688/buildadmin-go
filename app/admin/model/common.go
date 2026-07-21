package model

import (
	"go-build-admin/app/pkg/header"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func IsSuperAdmin(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		adminAuth := header.GetAdminAuth(ctx)
		if !adminAuth.IsSuperAdmin {
			return db.Where("admin_id = ?", adminAuth.Id)
		}
		return db
	}
}
