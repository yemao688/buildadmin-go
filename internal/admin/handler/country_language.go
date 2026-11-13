package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/common/country"
	"buildadmin-go/internal/common/translate"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type CountryLanguageHandler struct {
	TableHandler[model.CountryLanguage, dto.CountryLanguageParam, int64]
	countryLanguageM *repository.CountryLanguageRepository
	countrySvc       *country.Service
	translate        *translate.Client
}

func NewCountryLanguageHandler(log *zap.Logger, countryLanguageM *repository.CountryLanguageRepository, countrySvc *country.Service, translate *translate.Client) *CountryLanguageHandler {
	return &CountryLanguageHandler{
		TableHandler: NewTableHandler[model.CountryLanguage, dto.CountryLanguageParam, int64](
			log,
			countryLanguageM,
			TableOptions[model.CountryLanguage, dto.CountryLanguageParam]{
				PKJSONName:  "id",
				PartialEdit: map[string]bool{},
				AfterMutation: func(ctx *gin.Context) {
					// 语言列表缓存失效：响应暂存成功后登记，请求事务提交后执行；非事务请求立即执行
					requesttx.InvalidateAfterMutation(ctx, countrySvc.InvalidateLanguageCache)
				},
			},
		),
		countryLanguageM: countryLanguageM,
		countrySvc:       countrySvc,
		translate:        translate,
	}
}

// GetMultTranslations 一键翻译多语言：接收源语言 lan 与待翻译文本 lan_value，
// 调用外部翻译服务把文本翻译为启用的全部 country_language 语言，返回
// [{lan, value}]。对齐 PHP 上游 country\Language::getMultTranslations。
func (h *CountryLanguageHandler) GetMultTranslations(ctx *gin.Context) {
	var params struct {
		Lan      string `json:"lan" binding:"required"`
		LanValue string `json:"lan_value" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	languages, err := h.countrySvc.EnabledLanguages(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	tos := make([]string, 0, len(languages))
	for _, lang := range languages {
		tos = append(tos, lang.Lan)
	}

	texts, err := h.translate.TranslateMulti(ctx, params.Lan, tos, params.LanValue)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	// 按启用语言顺序输出（与 PHP 以 lan_tos 顺序遍历响应一致），保持确定性。
	list := make([]map[string]string, 0, len(tos))
	for _, lan := range tos {
		if value, ok := texts[lan]; ok {
			list = append(list, map[string]string{"lan": lan, "value": value})
		}
	}
	response.Success(ctx, list)
}

// NoNeedPermissionActions 声明需登录但免权限的 action。getmulttranslations
// 是多语言表单组件的通用翻译工具接口：任何模块的表单（country_language 之外）
// 都会调用它，不归属某一张业务菜单，故显式豁免 admin_rule 校验（仍需登录）。
func (h *CountryLanguageHandler) NoNeedPermissionActions() []string {
	return []string{"getmulttranslations"}
}
