package handler

import (
	"go-build-admin/app/common/member"
	"go-build-admin/app/pkg/clickcaptcha"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/validator"
	"go-build-admin/conf"
	"regexp"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserHandler struct {
	log          *zap.Logger
	config       *conf.Configuration
	authM        *member.Service
	clickCaptcha *clickcaptcha.ClickCaptcha
}

func NewUserHandler(log *zap.Logger, config *conf.Configuration, authM *member.Service, clickCaptcha *clickcaptcha.ClickCaptcha) *UserHandler {
	return &UserHandler{log: log, config: config, authM: authM, clickCaptcha: clickCaptcha}
}

type Login struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required,password"`
	Keep        bool   `json:"keep"`
	CaptchaId   string `json:"captchaId"`
	CaptchaInfo string `json:"captchaInfo"`
}

func (v Login) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"username.required": "username required",
		"password.required": "password required",
		"password.password": "password invalid",
	}
}

type Register struct {
	Username    string `json:"username" binding:"required,min=3,max=16"`
	Password    string `json:"password" binding:"required,password"`
	CaptchaId   string `json:"captchaId" binding:"required"`
	CaptchaInfo string `json:"captchaInfo" binding:"required"`
}

func (v Register) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"username.required":    "username required",
		"username.min":         "username invalid",
		"username.max":         "username invalid",
		"password.required":    "password required",
		"password.password":    "password invalid",
		"captchaId.required":   "captcha required",
		"captchaInfo.required": "captcha required",
	}
}

func (h *UserHandler) Login(ctx *gin.Context) {
	var params Login
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if h.config.App.UserLoginCaptcha && !h.clickCaptcha.Check(params.CaptchaId, params.CaptchaInfo, true) {
		FailByErr(ctx, cErr.BadRequest("Captcha error"))
		return
	}
	result, err := h.authM.Login(ctx, params.Username, params.Password, params.Keep)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"userInfo":  result,
		"routePath": "/user",
	})
}

func (h *UserHandler) Register(ctx *gin.Context) {
	var params Register
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,15}$`).MatchString(params.Username) {
		FailByErr(ctx, cErr.BadRequest("username invalid"))
		return
	}
	if !h.clickCaptcha.Check(params.CaptchaId, params.CaptchaInfo, true) {
		FailByErr(ctx, cErr.BadRequest("Captcha error"))
		return
	}
	result, err := h.authM.Register(ctx, params.Username, params.Password)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]interface{}{
		"userInfo":  result,
		"routePath": "/user",
	})
}

func (h *UserHandler) Logout(ctx *gin.Context) {
	var params struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, err)
		return
	}
	if err := h.authM.Logout(ctx, params.RefreshToken); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
