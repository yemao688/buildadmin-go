package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/header"
	"go-build-admin/utils"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type AdminInfoHandler struct {
	Base
	log    *zap.Logger
	adminM *model.AdminModel
	authM  *model.AuthModel
}

func NewAdminInfoHandler(log *zap.Logger, adminM *model.AdminModel, authM *model.AuthModel) *AdminInfoHandler {
	return &AdminInfoHandler{
		Base:   Base{currentM: adminM},
		log:    log,
		adminM: adminM,
		authM:  authM,
	}
}

func (h *AdminInfoHandler) Index(ctx *gin.Context) {
	adminAuth := header.GetAdminAuth(ctx)
	admin, err := h.authM.GetInfo(ctx, adminAuth.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"info": map[string]interface{}{
			"id":              admin.ID,
			"username":        admin.Username,
			"nickname":        admin.Nickname,
			"avatar":          admin.Avatar,
			"email":           admin.Email,
			"mobile":          admin.Mobile,
			"motto":           admin.Motto,
			"last_login_time": utils.FormatFromUnixTime(admin.LastLoginTime),
			"token":           adminAuth.Token,
			"refresh_token":   "",
		},
	})
}

func (h *AdminInfoHandler) Edit(ctx *gin.Context) {
	var params = struct {
		IDS
		Admin
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	admin, err := h.adminM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	avatar := ctx.Request.FormValue("avatar")
	if avatar != "" {
		admin.Avatar = avatar
		groupArr := []string{}
		err = h.adminM.Edit(ctx, admin, "username, last_login_time, password, salt, status", groupArr)
		if err != nil {
			FailByErr(ctx, err)
			return
		}
		Success(ctx, "")
		return
	}

	if params.Password != "" {
		if err := h.adminM.ResetPassword(ctx, admin.ID, params.Password); err != nil {
			FailByErr(ctx, err)
			return
		}
	}

	copier.Copy(&admin, params)
	err = h.adminM.Edit(ctx, admin, "username, last_login_time, password, salt, status", params.GroupArr)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
