package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/common/country"
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
}

func NewCountryLanguageHandler(log *zap.Logger, countryLanguageM *repository.CountryLanguageRepository, countrySvc *country.Service) *CountryLanguageHandler {
	return &CountryLanguageHandler{Base: Base{currentM: countryLanguageM}, log: log, countryLanguageM: countryLanguageM, countrySvc: countrySvc}
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
