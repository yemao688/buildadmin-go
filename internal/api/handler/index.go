package handler

import (
	"buildadmin-go/internal/api/service"
	"buildadmin-go/internal/common/country"
	"buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/util"

	"github.com/gin-gonic/gin"
)

// IndexHandler 前台/会员中心初始化接口（对齐 PHP app/api/controller/Index.php
// 的 index 动作）：返回全站 site 配置、登录态 userInfo、语言与币种列表。
// 路由本身免登录（public 集合），userInfo 仅在请求携带有效 ba-user-token 时返回。
type IndexHandler struct {
	config  *conf.Configuration
	configM *siteconfig.Service
	country *country.Service
	authM   *service.MemberService
}

func NewIndexHandler(config *conf.Configuration, configM *siteconfig.Service, countryService *country.Service, authM *service.MemberService) *IndexHandler {
	return &IndexHandler{config: config, configM: configM, country: countryService, authM: authM}
}

func (h *IndexHandler) Index(ctx *gin.Context) {
	basicConfig, err := h.configM.GetKVByGroup(ctx, "basics")
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	languages, err := h.country.EnabledLanguages(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	// 默认语言 = 启用的语言第一条（与 country.Service.DefaultLan 语义一致），
	// 空表兜底 "en"；复用上面的 languages 查询，避免重复查库。
	defaultLan := "en"
	if len(languages) > 0 {
		defaultLan = languages[0].Lan
	}
	currencies, err := h.country.EnabledCurrencies(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	uploadConfig, err := upload.UploadSiteConfig(ctx, h.configM, h.config)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	// cdnUrl 三段链（对齐 PHP full_url）：静态 cdn_url → alioss upload cdn → 协议兜底
	uploadCDN, _ := uploadConfig["cdn"].(string)
	userInfo, _ := h.authM.UserInfoByToken(ctx.Request.Header.Get("ba-user-token"))

	response.Success(ctx, map[string]any{
		"site": map[string]any{
			"siteName":     basicConfig["site_name"],
			"version":      basicConfig["version"],
			"cdnUrl":       util.FullUrl("", h.config.App.CdnUrl, uploadCDN, util.GetBaseURL(ctx), ""),
			"upload":       uploadConfig,
			"recordNumber": basicConfig["record_number"],
			"cdnUrlParams": h.config.App.CdnUrlParams,
		},
		"userInfo": userInfo,
		"language": languages,
		"default_language": defaultLan,
		"currency": currencies,
	})
}
