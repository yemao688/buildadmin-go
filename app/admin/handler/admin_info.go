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

type SelfAdmin struct {
	Nickname *string `json:"nickname" binding:"omitempty,required"`
	Avatar   *string `json:"avatar" binding:""`
	Email    string  `json:"email" binding:"omitempty,email"`
	Mobile   string  `json:"mobile" binding:"omitempty,phone"`
	Password string  `json:"password" binding:"omitempty,password"`
	Motto    string  `json:"motto"`
}

func (v SelfAdmin) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"email.email":       "email error",
		"mobile.phone":      "mobile error",
		"password.password": "password invalid",
	}
}

func (h *AdminInfoHandler) Edit(ctx *gin.Context) {
	var params = struct {
		SelfAdmin
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	admin, err := h.adminM.GetOne(ctx, adminAuth.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	//判读是否 只更新头像
	if params.Avatar != nil {
		admin.Avatar = *params.Avatar
		err = h.adminM.SelfEdit(ctx, admin, []string{"avatar"})
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
	err = h.adminM.SelfEdit(ctx, admin, []string{"nickname", "email", "mobile", "motto"})
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
