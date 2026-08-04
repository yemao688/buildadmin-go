package handler

import (
	dto "buildadmin-go/internal/admin/dto"
	repository "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/response"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type CountryLanguageContentHandler struct {
	Base
	log                     *zap.Logger
	countryLanguageContentM *repository.CountryLanguageContentRepository
}

func NewCountryLanguageContentHandler(log *zap.Logger, countryLanguageContentM *repository.CountryLanguageContentRepository) *CountryLanguageContentHandler {
	return &CountryLanguageContentHandler{Base: Base{currentM: countryLanguageContentM}, log: log, countryLanguageContentM: countryLanguageContentM}
}

func (h *CountryLanguageContentHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		response.Success(ctx, data)
	}
	list, total, err := h.countryLanguageContentM.List(ctx)
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

func (h *CountryLanguageContentHandler) Add(ctx *gin.Context) {
	var params dto.CountryLanguageContentParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.CountryLanguageContent
	copier.Copy(&data, params)
	err := h.countryLanguageContentM.Add(ctx, data)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *CountryLanguageContentHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{}) {
		return
	}

	type CountryLanguageContentIDs struct {
		ID int64 `json:"id" binding:"required"`
	}
	var params = struct {
		CountryLanguageContentIDs
		dto.CountryLanguageContentParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		response.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.countryLanguageContentM.GetOne(ctx, params.ID)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.countryLanguageContentM.Edit(ctx, data)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.Success(ctx, "")
}

func (h *CountryLanguageContentHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int64 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		response.FailByErr(ctx, validator.GetError(param, err))
		return
	}
	err := h.countryLanguageContentM.Del(ctx, param.Ids)
	if err != nil {
		response.FailByErr(ctx, err)
		return
	}
	response.SuccessWithMessage(ctx, "Deleted successfully")
}
