package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"github.com/unknwon/com"
	"go.uber.org/zap"
)

type AdminGroupHandler struct {
	Base
	log         *zap.Logger
	adminGroupM *model.AdminGroupModel
	authM       *model.AuthModel
}

func NewAdminGroupHandler(log *zap.Logger, adminGroupM *model.AdminGroupModel, authM *model.AuthModel) *AdminGroupHandler {
	return &AdminGroupHandler{
		Base:        Base{currentM: adminGroupM},
		log:         log,
		adminGroupM: adminGroupM,
		authM:       authM,
	}
}

func (h *AdminGroupHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
	}

	result, total, err := h.adminGroupM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}

type AdminGroup struct {
}

func (v AdminGroup) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"username.min":      "username>2 and username<15",
		"username.max":      "username>2 and username<15",
		"email.email":       "email error",
		"mobile.phone":      "mobile error",
		"password.password": "password invalid",
	}
}

func (h *AdminGroupHandler) Add(ctx *gin.Context) {
	var params AdminRule
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.adminGroupM.Add(ctx, admin, params.GroupArr)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminGroupHandler) Edit(ctx *gin.Context) {
	id := com.StrTo(ctx.Request.FormValue("id")).MustInt()
	admin, err := h.adminGroupM.GetOne(ctx, int32(id))
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//校验数据权限
	if !h.CheckDataLimit(ctx, admin.ID) {
		FailByErr(ctx, cErr.BadRequest("you have no permission"))
		return
	}

	if ctx.Request.Method == http.MethodGet {
		Success(ctx, map[string]interface{}{
			"row": admin,
		})
		return
	}

	type AdminEdit struct {
		IDS
		Admin
	}
	var params AdminEdit
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	authAdmin := header.GetAdminAuth(ctx)
	if authAdmin.Id == admin.ID && params.Status == "0" {
		FailByErr(ctx, cErr.BadRequest("please use another administrator account to disable the current account!"))
		return
	}

	if params.Password != "" {
		if err := h.adminGroupM.ResetPassword(ctx, admin.ID, params.Password); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	copier.Copy(&admin, params)
	err = h.adminGroupM.Edit(ctx, admin, params.GroupArr)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminGroupHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.adminGroupM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

// 获取菜单列表
func (h *AdminGroupHandler) GetMenus(ctx *gin.Context, groups []string, id int32) {

	return nil
}
