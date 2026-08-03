package handler

import (
	adminauth "buildadmin-go/internal/admin/repository"
	routinemodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/pkg/validator"
	"buildadmin-go/internal/common/country"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/clickcaptcha"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/utils"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type IndexHandler struct {
	config       *conf.Configuration
	log          *zap.Logger
	authM        *adminauth.AuthRepository
	configM      *routinemodel.ConfigRepository
	country      *country.Service
	clickCaptcha *clickcaptcha.ClickCaptcha
}

func NewIndexHandler(config *conf.Configuration, log *zap.Logger, authM *adminauth.AuthRepository, configM *routinemodel.ConfigRepository, countryService *country.Service, clickCaptcha *clickcaptcha.ClickCaptcha) *IndexHandler {
	return &IndexHandler{config: config, log: log, authM: authM, configM: configM, country: countryService, clickCaptcha: clickCaptcha}
}

func (h *IndexHandler) Index(ctx *gin.Context) {
	info := header.GetAdminAuth(ctx)
	adminInfo, _ := h.authM.GetInfo(ctx, info.Id)

	menus, _ := h.authM.GetMenus(ctx, info.Id)
	if len(menus) == 0 {
		FailByErr(ctx, cErr.BadRequest("No background menu, please contact super administrator!"))
		return
	}

	basicConfig, err := h.configM.GetKVByGroup(ctx, "basics")
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	languages, err := h.country.EnabledLanguages(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	languageTabs := make([]map[string]string, 0, len(languages))
	for _, language := range languages {
		languageTabs = append(languageTabs, map[string]string{"lan": language.Lan, "remark": language.Remark})
	}

	uploadConfig, err := upload.UploadSiteConfig(ctx, h.configM, h.config)
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
		"menus":        menus,
		"languageTabs": languageTabs,
		"siteConfig": map[string]any{
			"siteName":         basicConfig["site_name"],
			"version":          basicConfig["version"],
			"cdnUrl":           utils.FullUrl("", h.config.App.CdnUrl, utils.GetBaseURL(ctx), ""),
			"apiUrl":           h.config.App.ApiUrl,
			"upload":           uploadConfig,
			"cdnUrlParams":     h.config.App.CdnUrlParams,
			"userLoginCaptcha": h.config.App.UserLoginCaptcha,
		},
		"terminal": map[string]any{
			"npmPackageManager":    h.config.Terminal.NpmPackageManager,
			"phpDevelopmentServer": false,
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

func (v Login) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{
		"username.min":      "username min>10",
		"username.max":      "username max<30",
		"password.required": "password invalid",
	}
}

func (h *IndexHandler) Login(ctx *gin.Context) {
	// 检查登录态
	if _, ok := h.authM.IsLogin(ctx); ok {
		FailByErr(ctx, cErr.BadRequest("You have already logged in. There is no need to log in again~", cErr.LoginResponseCode))
		return
	}

	needCaptcha := h.config.App.AdminLoginCaptcha
	if ctx.Request.Method == http.MethodPost {
		var params Login
		if err := ctx.ShouldBindJSON(&params); err != nil {
			FailByErr(ctx, validator.GetError(params, err))
			return
		}

		if needCaptcha {
			if params.CaptchaId == "" || params.CaptchaInfo == "" {
				FailByErr(ctx, cErr.BadRequest("Captcha error"))
				return
			}

			if !h.clickCaptcha.Check(params.CaptchaId, params.CaptchaInfo, true) {
				FailByErr(ctx, cErr.BadRequest("Captcha error"))
				return
			}
		}
		ctx.Set("log_title", utils.Lang(ctx, "login", nil))

		result, err := h.authM.Login(ctx, params.Username, params.Password, params.Keep)
		if err != nil {
			FailByErr(ctx, err)
			return
		}
		loginResult, ok := result.(map[string]interface{})
		if !ok {
			FailByErr(ctx, cErr.InternalServer("Invalid login result"))
			return
		}
		adminID, ok := loginResult["id"].(int32)
		if !ok || adminID == 0 {
			FailByErr(ctx, cErr.InternalServer("Invalid login result"))
			return
		}
		loginToken, _ := loginResult["token"].(string)
		username, _ := loginResult["username"].(string)
		header.SetAdminAuth(ctx, header.AdminAuth{
			Language: ctx.GetHeader("Accept-Language"),
			IsLogin:  true,
			Id:       adminID,
			Username: username,
			Token:    loginToken,
		})
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
	RefreshToken string `json:"refreshToken"`
}

func (v Logout) GetMessages() validator.ValidatorMessages {
	return validator.ValidatorMessages{}
}

func (h *IndexHandler) Logout(ctx *gin.Context) {
	var params Logout
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	err := h.authM.Logout(ctx, params.RefreshToken)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
