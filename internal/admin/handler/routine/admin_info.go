package routine

import (
	adminhandler "go-build-admin/internal/admin/handler"
	adminmodel "go-build-admin/internal/admin/model/auth"
	"go-build-admin/internal/admin/validate"
	"go-build-admin/internal/pkg/header"
	"go-build-admin/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type AdminInfoHandler struct {
	adminhandler.Base
	log    *zap.Logger
	adminM *adminmodel.AdminModel
	authM  *adminmodel.AuthModel
}

func NewAdminInfoHandler(log *zap.Logger, adminM *adminmodel.AdminModel, authM *adminmodel.AuthModel) *AdminInfoHandler {
	return &AdminInfoHandler{
		Base:   adminhandler.NewBase(adminM),
		log:    log,
		adminM: adminM,
		authM:  authM,
	}
}

func (h *AdminInfoHandler) Index(ctx *gin.Context) {
	adminAuth := header.GetAdminAuth(ctx)
	admin, err := h.authM.GetInfo(ctx, adminAuth.Id)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, map[string]any{
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
		adminhandler.FailByErr(ctx, validate.GetError(params, err))
		return
	}

	adminAuth := header.GetAdminAuth(ctx)
	admin, err := h.adminM.GetOne(ctx, adminAuth.Id)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	//判读是否 只更新头像
	if params.Avatar != nil {
		admin.Avatar = *params.Avatar
		err = h.adminM.SelfEdit(ctx, admin, []string{"avatar"})
		if err != nil {
			adminhandler.FailByErr(ctx, err)
			return
		}
		adminhandler.Success(ctx, "")
		return
	}

	if params.Password != "" {
		if err := h.adminM.ResetPassword(ctx, admin.ID, params.Password); err != nil {
			adminhandler.FailByErr(ctx, err)
			return
		}
	}

	copier.Copy(&admin, params)
	err = h.adminM.SelfEdit(ctx, admin, []string{"nickname", "email", "mobile", "motto"})
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}
