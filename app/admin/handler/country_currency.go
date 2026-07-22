package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type CountryCurrencyHandler struct {
	Base
	log              *zap.Logger
	countryCurrencyM *model.CountryCurrencyModel
}

func NewCountryCurrencyHandler(log *zap.Logger, countryCurrencyM *model.CountryCurrencyModel) *CountryCurrencyHandler {
	return &CountryCurrencyHandler{Base: Base{currentM: countryCurrencyM}, log: log, countryCurrencyM: countryCurrencyM}
}

func (h *CountryCurrencyHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	list, total, err := h.countryCurrencyM.List(ctx)
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

type CountryCurrencyParam struct {
	Code   string               `json:"code"`   // 货币代码
	Name   string               `json:"name"`   // 货币名称
	Symbol string               `json:"symbol"` // 货币符号
	Rate   validate.FlexFloat64 `json:"rate"`   // 汇率
	Status validate.FlexInt32   `json:"status"` // 状态:0=禁用,1=启用
	Weigh  validate.FlexInt32   `json:"weigh"`  // 权重
}

func (h *CountryCurrencyHandler) Add(ctx *gin.Context) {
	var params CountryCurrencyParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.CountryCurrency
	copier.Copy(&data, params)
	err := h.countryCurrencyM.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *CountryCurrencyHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		return
	}

	type CountryCurrencyIDs struct {
		ID int64 `json:"id" binding:"required"`
	}
	var params = struct {
		CountryCurrencyIDs
		CountryCurrencyParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.countryCurrencyM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.countryCurrencyM.Edit(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *CountryCurrencyHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int64 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.countryCurrencyM.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
