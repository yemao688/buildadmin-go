package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type AdminHandler struct {
	Base
	log    *zap.Logger
	adminM *model.AdminModel
	authM  *model.AuthModel
}

func NewAdminHandler(log *zap.Logger, adminM *model.AdminModel, authM *model.AuthModel) *AdminHandler {
	return &AdminHandler{
		log:    log,
		adminM: adminM,
		authM:  authM,
	}
}

func (h *AdminHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}

	result, total, err := h.adminM.List(ctx)
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

func (h *AdminHandler) Add(ctx *gin.Context) {
	var params validate.Admin
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var admin model.Admin
	copier.Copy(&admin, params)
	err := h.adminM.Add(ctx, admin)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminHandler) Edit(ctx *gin.Context) {
	var params validate.Admin
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	var admin model.Admin
	copier.Copy(&admin, params)
	err := h.adminM.Edit(ctx, admin)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *AdminHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	err := h.adminM.Del(ctx, params.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

// 检查分组权限
// func (h *AdminHandler) CheckGroupAuth(ctx *gin.Context) {
// 	result, err := h.authM.IsSuperAdmin(ctx, 1)
// 	if err != nil {
// 		FailByErr(ctx, err)
// 		return
// 	}
// 	Success(ctx, result)
// }
