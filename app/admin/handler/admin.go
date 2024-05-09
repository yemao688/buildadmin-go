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
		return
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

type Admin struct {
	Username string `json:"username" binding:"required"`
	Nickname string `json:"nickname" binding:"required"`
	Avatar   string `json:"avatar" binding:""`
	Email    string `json:"email" binding:"required"`
	Mobile   string `json:"mobile" binding:"required"`
	Password string `json:"password" binding:"required"`
	Motto    string `json:"motto" binding:""`
	Status   string `json:"status" binding:""`
	GroupArr string `json:"group_arr" binding:"required"`
}

func (v Admin) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *AdminHandler) Add(ctx *gin.Context) {
	var params Admin
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
// func (h *AdminHandler) CheckGroupAuth(ctx *gin.Context,groups []) error {
// 	adminAuth := header.GetAdminAuth(ctx)
// 	if ok := h.authM.IsSuperAdmin(ctx, adminAuth.Id); ok {
// 		return nil
// 	}

// 	authGroups := h.authM.GetAllAuthGroups("allAuthAndOthers")
// 	for _, v := range authGroups {
// 		if

// 	}

// }
