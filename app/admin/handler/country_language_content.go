package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type CountryLanguageContentHandler struct {
	Base
	log                     *zap.Logger
	countryLanguageContentM *model.CountryLanguageContentModel
}

func NewCountryLanguageContentHandler(log *zap.Logger, countryLanguageContentM *model.CountryLanguageContentModel) *CountryLanguageContentHandler {
	return &CountryLanguageContentHandler{Base: Base{currentM: countryLanguageContentM}, log: log, countryLanguageContentM: countryLanguageContentM}
}

func (h *CountryLanguageContentHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	list, total, err := h.countryLanguageContentM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":   list,
		"total":  total,
		"remark": "",
	})
}

type CountryLanguageContentParam struct {
	Lan   string             `json:"lan"`   // 语言代码
	Group string             `json:"group"` // 分组
	Key   string             `json:"key"`   // 键
	Type  validate.FlexInt32 `json:"type"`  // 类型:0=文本,1=富文本,2=图片
	Value string             `json:"value"` // 值
}

func (h *CountryLanguageContentHandler) Add(ctx *gin.Context) {
	var params CountryLanguageContentParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.CountryLanguageContent
	copier.Copy(&data, params)
	err := h.countryLanguageContentM.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *CountryLanguageContentHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		return
	}

	type CountryLanguageContentIDs struct {
		ID int64 `json:"id" binding:"required"`
	}
	var params = struct {
		CountryLanguageContentIDs
		CountryLanguageContentParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.countryLanguageContentM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.countryLanguageContentM.Edit(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *CountryLanguageContentHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int64 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.countryLanguageContentM.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
