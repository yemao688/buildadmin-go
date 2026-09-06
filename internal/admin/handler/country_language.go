package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/common/country"
	"buildadmin-go/internal/common/translate"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type CountryLanguageHandler struct {
	Base
	log              *zap.Logger
	countryLanguageM *repository.CountryLanguageRepository
	countrySvc       *country.Service
	translate        *translate.Client
}

func NewCountryLanguageHandler(log *zap.Logger, countryLanguageM *repository.CountryLanguageRepository, countrySvc *country.Service, translate *translate.Client) *CountryLanguageHandler {
	return &CountryLanguageHandler{Base: Base{currentM: countryLanguageM}, log: log, countryLanguageM: countryLanguageM, countrySvc: countrySvc, translate: translate}
}

func (h *CountryLanguageHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		response.Success(ctx, data)
		return
	}
	list, total, err := h.countryLanguageM.List(ctx)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, map[string]any{
		"list":   list,
		"total":  total,
		"remark": "",
	})
}

func (h *CountryLanguageHandler) Add(ctx *gin.Context) {
	var params dto.CountryLanguageParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.CountryLanguage
	copier.Copy(&data, params)
	err := h.countryLanguageM.Add(ctx, data)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	// 语言列表缓存失效：响应暂存成功后登记，请求事务提交后执行；非事务请求立即执行
	requesttx.InvalidateAfterMutation(ctx, h.countrySvc.InvalidateLanguageCache)
}

func (h *CountryLanguageHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{}) {
		return
	}

	type CountryLanguageIDs struct {
		ID int64 `json:"id" binding:"required"`
	}
	var params = struct {
		CountryLanguageIDs
		dto.CountryLanguageParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.countryLanguageM.GetOne(ctx, params.ID)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.countryLanguageM.Edit(ctx, data)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
	requesttx.InvalidateAfterMutation(ctx, h.countrySvc.InvalidateLanguageCache)
}

func (h *CountryLanguageHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int64 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		response.FailByErr(ctx, validator.GetError(param, err))
		return
	}
	err := h.countryLanguageM.Del(ctx, param.Ids)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.SuccessWithMessage(ctx, "Deleted successfully")
	requesttx.InvalidateAfterMutation(ctx, h.countrySvc.InvalidateLanguageCache)
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
