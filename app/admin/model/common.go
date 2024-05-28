package model

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"go-build-admin/app/pkg/header"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
		if value != nil {
			dataLimitAdminIds := value.([]string)
			if len(dataLimitAdminIds) > 0 {
				db.Where(" admin_id in ? ", dataLimitAdminIds)
			}
		}
		return db
	}
}

// 多个id
type MulIds []int32

func (j *MulIds) Scan(value interface{}) error {
	content, ok := value.(string)
	if !ok {
		return errors.New(fmt.Sprint("Failed MulIds value:", value))
	}

	ids := strings.Split(content, ",")
	result := []int32{}
	for _, v := range ids {
		num, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		result = append(result, int32(num))
	}
	*j = MulIds(result)
	return nil
}

func (j MulIds) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	result := []string{}
	for _, v := range j {
		result = append(result, strconv.Itoa(int(v)))
	}
	return strings.Join(result, ","), nil
}
