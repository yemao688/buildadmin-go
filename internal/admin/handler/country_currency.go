package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type CountryCurrencyHandler struct {
	Base
	log              *zap.Logger
	countryCurrencyM *repository.CountryCurrencyRepository
}

func NewCountryCurrencyHandler(log *zap.Logger, countryCurrencyM *repository.CountryCurrencyRepository) *CountryCurrencyHandler {
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

func (h *CountryCurrencyHandler) Add(ctx *gin.Context) {
	var params dto.CountryCurrencyParam
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
	if h.MaybePartialEdit(ctx, map[string]bool{}) {
		return
	}

	type CountryCurrencyIDs struct {
		ID int64 `json:"id" binding:"required"`
	}
	var params = struct {
		CountryCurrencyIDs
		dto.CountryCurrencyParam
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
		FailByErr(ctx, validator.GetError(param, err))
		return
	}
	err := h.countryCurrencyM.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	SuccessWithMessage(ctx, "Deleted successfully")
}
