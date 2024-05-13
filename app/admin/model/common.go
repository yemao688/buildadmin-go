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
		authAdmin := header.GetAdminAuth(ctx)
		if !authAdmin.IsSuperAdmin {
			db.Where(" admin_id = ? ", 1)
		}
		return db
	}
}

func LimitAdminIds(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		value, _ := ctx.Get("dataLimitAdminIds")
		if value != nil {
			dataLimitAdminIds := value.([]string)
			if len(dataLimitAdminIds) > 0 {
				db.Where(" admin_id in ? ", dataLimitAdminIds)
			}
		}
		return db
	}
}
