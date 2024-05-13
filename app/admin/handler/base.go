package handler

import (
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Base struct {
	currentM CommonModel
}

func (h *Base) Select(ctx *gin.Context) (interface{}, bool) {
	return nil, false
}

func (h *Base) CheckDataLimit(ctx *gin.Context, id int32) bool {
	value, _ := ctx.Get("dataLimitAdminIds")
	if value != nil {
		dataLimitAdminIds := value.([]int32)
		if len(dataLimitAdminIds) == 0 {
			return true
		}

		ok := false
		for _, v := range dataLimitAdminIds {
			if v == id {
				ok = true
				break
			}
		}
		return ok
	}
	return true
}

func (h *Base) Sortable(ctx *gin.Context) {
	type Sort struct {
		Id       int `json:"id" binding:"required"`
		TargetId int `json:"targetId" binding:"required"`
	}
	params := Sort{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if err := Sortable(ctx, h.currentM, params.Id, params.TargetId); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

type CommonModel interface {
	DB() *gorm.DB
	Table() string
}

func Sortable(ctx *gin.Context, m1 CommonModel, id, targetId int) error {
	type Row struct {
		Id    int
		Weigh int
	}
	source := Row{}
	target := Row{}
	err1 := m1.DB().Table(m1.Table()).Scopes(LimitAdminIds(ctx)).Where("id=?", id).Scan(&source).Error
	err2 := m1.DB().Table(m1.Table()).Scopes(LimitAdminIds(ctx)).Where("id=?", targetId).Scan(&target).Error
	if err1 != nil || err2 != nil {
		return cErr.BadRequest("record not found")
	}

	if source.Weigh == target.Weigh {
		return cErr.BadRequest("invalid collation because the weights of the two targets are equal")
	}

	m1.DB().Table(m1.Table()).Scopes(LimitAdminIds(ctx)).Where("id=?", id).Updates(map[string]interface{}{
		"weigh": target.Weigh,
	})
	m1.DB().Table(m1.Table()).Scopes(LimitAdminIds(ctx)).Where("id=?", targetId).Updates(map[string]interface{}{
		"weigh": source.Weigh,
	})
	return nil
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
