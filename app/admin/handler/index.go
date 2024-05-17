package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/clickcaptcha"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/conf"
	"go-build-admin/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type IndexHandler struct {
	config       *conf.Configuration
	log          *zap.Logger
	authM        *model.AuthModel
	configM      *model.ConfigModel
	clickCaptcha *clickcaptcha.ClickCaptcha
}

func NewIndexHandler(config *conf.Configuration, log *zap.Logger, authM *model.AuthModel, configM *model.ConfigModel, clickCaptcha *clickcaptcha.ClickCaptcha) *IndexHandler {
	return &IndexHandler{config: config, log: log, authM: authM, configM: configM, clickCaptcha: clickCaptcha}
}

func (h *IndexHandler) Index(ctx *gin.Context) {
	info := header.GetAdminAuth(ctx)
	adminInfo, _ := h.authM.GetInfo(ctx, info.Id)

	menus, _ := h.authM.GetMenus(ctx, 1)
	if len(menus) == 0 {
		FailByErr(ctx, cErr.BadRequest("No background menu, please contact super administrator!"))
		return
	}

	version, err := h.configM.GetValueByName(ctx, "version")
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	Success(ctx, map[string]any{
		"adminInfo": map[string]any{
			"id":              adminInfo.ID,
			"username":        adminInfo.Username,
			"nickname":        adminInfo.Nickname,
			"avatar":          utils.DefaultUrl(adminInfo.Avatar, h.config.App.DefaultAvatar),
			"last_login_time": utils.FormatFromUnixTime(adminInfo.LastLoginTime),
			"super":           h.authM.IsSuperAdmin(info.Id),
		},
		"menus": menus,
		"siteConfig": map[string]any{
			"siteName": h.config.App.AppName,
			"version":  version,
			"cdnUrl":   utils.FullUrl("", h.config.App.CdnUrl, ctx.Request.Host, ""),
			"apiUrl":   h.config.App.ApiUrl,
			"upload":   h.config.Upload,
		},
		"terminal": map[string]any{
			"installServicePort": h.config.Terminal.InstallServicePort,
			"npmPackageManager":  h.config.Terminal.NpmPackageManager,
		},
	})
}

type Login struct {
	Username    string `json:"username" binding:"required,min=3,max=30"`
	Password    string `json:"password" binding:"required,password"`
	Keep        bool   `json:"keep"`
	CaptchaId   string `json:"captchaId"`
	CaptchaInfo string `json:"captchaInfo"`
}

func (v Login) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"username.min":   "username min>10",
		"username.max":   "username max<30",
		"password.regex": "password invalid",
	}
}

func (h *IndexHandler) Login(ctx *gin.Context) {
	// 检查登录态
	if _, ok := h.authM.IsLogin(ctx); ok {
		FailByErr(ctx, cErr.BadRequest("you have already logged in. There is no need to log in again~", cErr.LOGIN_RESPONSE_CODE))
	}

	needCaptcha := h.config.App.AdminLoginCaptcha
	if ctx.Request.Method == "POST" {
		var params Login
		if err := ctx.ShouldBindJSON(&params); err != nil {
			FailByErr(ctx, validate.GetError(params, err))
			return
		}

		if needCaptcha {
			if params.CaptchaId == "" || params.CaptchaInfo == "" {
				FailByErr(ctx, cErr.BadRequest("need captcha"))
				return
			}

			if !h.clickCaptcha.Check(params.CaptchaId, params.CaptchaInfo, true) {
				FailByErr(ctx, cErr.BadRequest("captcha error"))
				return
			}
		}
		ctx.Set("log_title", utils.Lang(ctx, "login", nil))

		result, err := h.authM.Login(ctx, params.Username, params.Password, params.Keep)
		if err != nil {
			FailByErr(ctx, err)
			return
		}
		Success(ctx, map[string]interface{}{
			"userInfo":  result,
			"routePath": "/admin",
		})
		return
	}

	Success(ctx, map[string]any{
		"captcha": needCaptcha,
	})

}

type Logout struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (v Logout) GetMessages() validate.ValidatorMessages {
	return validate.ValidatorMessages{
		"refreshToken.required": "not content",
	}
}

func (h *IndexHandler) Logout(ctx *gin.Context) {
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
