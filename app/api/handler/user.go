package handler

import (
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/common/model"
	"go-build-admin/app/pkg/captcha"
	"go-build-admin/app/pkg/clickcaptcha"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/conf"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type UserHandler struct {
	log          *zap.Logger
	config       *conf.Configuration
	authM        *model.AuthModel
	clickCaptcha *clickcaptcha.ClickCaptcha
	captcha      *captcha.Captcha
}

func NewUserHandler(log *zap.Logger, config *conf.Configuration, authM *model.AuthModel, clickCaptcha *clickcaptcha.ClickCaptcha, captcha *captcha.Captcha) *UserHandler {
	return &UserHandler{log: log, config: config, authM: authM, clickCaptcha: clickCaptcha, captcha: captcha}
}

type Login struct {
	Tab          string `json:"tab"`
	Email        string `json:"email"`
	Mobile       string `json:"mobile"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	Keep         bool   `json:"keep"`
	Captcha      string `json:"captcha"`
	CaptchaId    string `json:"captchaId"`
	CaptchaInfo  string `json:"captchaInfo"`
	RegisterType string `json:"registerType"`
}

func (v Login) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *UserHandler) CheckIn(ctx *gin.Context) {
	openMemberCenter := h.config.App.OpenMemberCenter
	if !openMemberCenter {
		FailByErr(ctx, cErr.BadRequest("Member center disabled"))
		return
	}

	//检查登陆
	if _, ok := h.authM.IsLogin(ctx); ok {
		FailByErr(ctx, cErr.BadRequest("You have already logged in. There is no need to log in again~", cErr.LOGIN_RESPONSE_CODE))
	}

	if ctx.Request.Method != http.MethodPost {
		Success(ctx, map[string][]string{
			"accountVerificationType": {"mobile", "email"},
		})
		return
	}

	var params Login
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if params.Tab == "login" {
		if !h.clickCaptcha.Check(params.CaptchaId, params.CaptchaInfo, true) {
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
		return

	} else if params.Tab == "register" {
		registerType := ""
		if params.RegisterType == "email" {
			registerType = "email" + "user_register"
		} else {
			registerType = "mobile" + "user_register"
		}

		if !h.captcha.Check(params.Captcha, registerType) {
			FailByErr(ctx, cErr.BadRequest("Please enter the correct verification code"))
			return
		}

		result, err := h.authM.Register(ctx, params.Username, params.Password, params.Mobile, params.Email)
		if err != nil {
			FailByErr(ctx, err)
			return
		}

		Success(ctx, map[string]interface{}{
			"userInfo":  result,
			"routePath": "/user",
		})
		return
	}

}

type Logout struct {
	RefreshToken string `json:"refreshToken"`
}

func (v Logout) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{}
}

func (h *UserHandler) Logout(ctx *gin.Context) {
	var params Logout
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	err := h.authM.Logout(ctx, params.RefreshToken)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
