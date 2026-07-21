package handler

import (
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/common/model"
	"go-build-admin/app/pkg/captcha"
	"go-build-admin/app/pkg/clickcaptcha"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/conf"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
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

func userRegisterCaptchaID(params Login) string {
	if params.RegisterType == "email" {
		return params.Email + "user_register"
	}
	return params.Mobile + "user_register"
}

type userRegisterValidation struct {
	RegisterType string `json:"registerType" binding:"required,oneof=email mobile"`
	Username     string `json:"username" binding:"required,min=3,max=16"`
	Password     string `json:"password" binding:"required,password"`
	Email        string `json:"email" binding:"required_if=RegisterType email,omitempty,email"`
	Mobile       string `json:"mobile" binding:"required_if=RegisterType mobile,omitempty,phone"`
	Captcha      string `json:"captcha" binding:"required"`
}

func (v userRegisterValidation) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"RegisterType.required": "registerType required",
		"RegisterType.oneof":    "registerType invalid",
		"Username.required":     "username required",
		"Username.min":          "username invalid",
		"Username.max":          "username invalid",
		"Password.required":     "password required",
		"Password.password":     "password invalid",
		"Email.required_if":     "email required",
		"Email.email":           "email invalid",
		"Mobile.required_if":    "mobile required",
		"Mobile.phone":          "mobile invalid",
		"Captcha.required":      "captcha required",
	}
}

func (h *UserHandler) CheckIn(ctx *gin.Context) {
	openMemberCenter := h.config.App.OpenMemberCenter
	if !openMemberCenter {
		FailByErr(ctx, cErr.BadRequest("Member center disabled"))
		return
	}

	//检查登陆
	if _, ok := h.authM.IsLogin(ctx); ok {
		FailByErrWithData(ctx, cErr.BadRequest("You have already logged in. There is no need to log in again~", cErr.LoginResponseCode), map[string]string{
			"type": "logged in",
		})
		return
	}

	if ctx.Request.Method != http.MethodPost {
		Success(ctx, map[string]interface{}{
			"accountVerificationType": []string{"mobile", "email"},
			"userLoginCaptchaSwitch":  h.config.App.UserLoginCaptcha,
		})
		return
	}

	var params Login
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}

	if params.Tab != "login" && params.Tab != "register" {
		FailByErr(ctx, cErr.BadRequest("tab invalid"))
		return
	}

	if params.Tab == "login" {
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
		return

	} else if params.Tab == "register" {
		registerParams := userRegisterValidation{
			RegisterType: params.RegisterType,
			Username:     params.Username,
			Password:     params.Password,
			Email:        params.Email,
			Mobile:       params.Mobile,
			Captcha:      params.Captcha,
		}
		if err := binding.Validator.ValidateStruct(registerParams); err != nil {
			FailByErr(ctx, validate.GetError(registerParams, err))
			return
		}
		if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{2,15}$`).MatchString(params.Username) {
			FailByErr(ctx, cErr.BadRequest("username invalid"))
			return
		}

		registerType := userRegisterCaptchaID(params)

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
