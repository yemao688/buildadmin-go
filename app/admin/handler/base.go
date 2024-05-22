package handler

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/utils"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"gorm.io/gorm"
)

type CommonModel interface {
	DB() *gorm.DB
	Table() string
}

type Base struct {
	currentM CommonModel
}

func (h *Base) Select(ctx *gin.Context) (interface{}, bool) {
	return nil, false
}

func (h *Base) CheckDataLimit(ctx *gin.Context, id int32) bool {
	value, exists := ctx.Get("dataLimitAdminIds")
	if exists && value != nil {
		dataLimitAdminIds := value.([]int32)
		if len(dataLimitAdminIds) == 0 {
			return true
		}
		return slices.Contains(dataLimitAdminIds, id)
	}
	return true
}

func (h *Base) One(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	fmt.Println(id)
	result := map[string]interface{}{}
	err := h.currentM.DB().Table(h.currentM.Table()).Where("id=?", id).Take(&result).Error
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	fmt.Println(result)

	//校验数据权限
	if !h.CheckDataLimit(ctx, int32(id)) {
		FailByErr(ctx, cErr.BadRequest("You have no permission"))
		return
	}

	Success(ctx, map[string]interface{}{
		"row": result,
	})
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
		return cErr.BadRequest("Record not found")
	}

	if source.Weigh == target.Weigh {
		return cErr.BadRequest("Invalid collation because the weights of the two targets are equal")
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

func (h *Base) GetRemark(ctx *gin.Context) string {
	var rule = struct {
		Remark string
	}{}
	name := ctx.Request.URL.Path
	name = strings.Replace(name, ".", "/", -1)
	name = strings.Replace(name, "/admin/", "", 1)
	slashIndex := strings.LastIndex(name, "/")

	nameArr := []string{name[:slashIndex], name[slashIndex+1:]}
	err := h.currentM.DB().Table(model.TableNameAdminRule).Where("name in ?", nameArr).Take(&rule).Error
	if err != nil {
		return ""
	}
	return utils.Lang(ctx, rule.Remark, nil)
}
