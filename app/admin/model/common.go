package model

import (
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
		if false {
			db.Where(" admin_id = ? ", 1)
		}
		return db
	}
}

func LimitAdminIds(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		dataLimitAdminIds := GetDataLimitAdminIds(ctx)
		if len(dataLimitAdminIds) > 0 {
			db.Where(" admin_id in ? ", dataLimitAdminIds)
		}
		return db
	}
}
