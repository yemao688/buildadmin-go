package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type CountryLanguageHandler struct {
	Base
	log              *zap.Logger
	countryLanguageM *model.CountryLanguageModel
}

func NewCountryLanguageHandler(log *zap.Logger, countryLanguageM *model.CountryLanguageModel) *CountryLanguageHandler {
	return &CountryLanguageHandler{Base: Base{currentM: countryLanguageM}, log: log, countryLanguageM: countryLanguageM}
}

func (h *CountryLanguageHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	list, total, err := h.countryLanguageM.List(ctx)
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

type CountryLanguageParam struct {
	Lan    string `json:"lan"`    // 语言代码
	Name   string `json:"name"`   // 语言名称
	Remark string `json:"remark"` // 备注
	Status int32  `json:"status"` // 状态:0=禁用,1=启用
	Weigh  int32  `json:"weigh"`  // 权重
}

func (h *CountryLanguageHandler) Add(ctx *gin.Context) {
	var params CountryLanguageParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.CountryLanguage
	copier.Copy(&data, params)
	err := h.countryLanguageM.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *CountryLanguageHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		return
	}

	type CountryLanguageIDs struct {
		ID int64 `json:"id" binding:"required"`
	}
	var params = struct {
		CountryLanguageIDs
		CountryLanguageParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.countryLanguageM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.countryLanguageM.Edit(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *CountryLanguageHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int64 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.countryLanguageM.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
