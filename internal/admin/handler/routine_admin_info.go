package handler

import (
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/util"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AdminInfoHandler struct {
	log   *zap.Logger
	authM *adminmodel.AuthRepository
	svc   *service.AdminService
}

func NewAdminInfoHandler(log *zap.Logger, authM *adminmodel.AuthRepository, svc *service.AdminService) *AdminInfoHandler {
	return &AdminInfoHandler{
		log:   log,
		authM: authM,
		svc:   svc,
	}
}

func (h *AdminInfoHandler) Index(ctx *gin.Context) {
	adminAuth := header.GetAdminAuth(ctx)
	admin, err := h.authM.GetInfo(ctx, adminAuth.Id)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]any{
		"info": map[string]interface{}{
			"id":              admin.ID,
			"username":        admin.Username,
			"nickname":        admin.Nickname,
			"avatar":          admin.Avatar,
			"email":           admin.Email,
			"mobile":          admin.Mobile,
			"motto":           admin.Motto,
			"last_login_time": util.FormatFromUnixTime(admin.LastLoginTime),
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

func (v SelfAdmin) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"email.email":       "email error",
		"mobile.phone":      "mobile error",
		"password.password": "password invalid",
	}
}

func (h *AdminInfoHandler) Edit(ctx *gin.Context) {
	var params SelfAdmin
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	actor, err := actorFromContext(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	adminAuth := header.GetAdminAuth(ctx)
	err = h.svc.UpdateSelf(ctx.Request.Context(), adminAuth.Id, service.SelfAdminParams{
		Nickname: params.Nickname,
		Avatar:   params.Avatar,
		Email:    params.Email,
		Mobile:   params.Mobile,
		Password: params.Password,
		Motto:    params.Motto,
	}, actor)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}
