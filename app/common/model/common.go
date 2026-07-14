package model

import (
	"go-build-admin/app/pkg/header"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Total(whereS string, whereP []interface{}, total *int64) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Count(total)
	}
}

func IsSuperAdmin(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		adminAuth := header.GetAdminAuth(ctx)
		if !adminAuth.IsSuperAdmin {
			db.Where(" admin_id = ? ", 1)
		}
		return db
	}
}

func LimitAdminIds(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		value, _ := ctx.Get("dataLimitAdminIds")
		if value == nil {
			return db
		}
		dataLimitAdminIds, ok := value.([]int32)
		if !ok || len(dataLimitAdminIds) == 0 {
			return db
		}
		return db.Where("admin_id IN ?", dataLimitAdminIds)
	}
}
