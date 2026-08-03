package handler

import (
	countrydto "buildadmin-go/internal/admin/dto"
	countrymodel "buildadmin-go/internal/admin/repository"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type LanguageHandler struct {
	Base
	log       *zap.Logger
	languageM *countrymodel.CountryLanguageRepository
}

func NewLanguageHandler(log *zap.Logger, languageM *countrymodel.CountryLanguageRepository) *LanguageHandler {
	return &LanguageHandler{Base: NewBase(languageM), log: log, languageM: languageM}
}

func (h *LanguageHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	list, total, err := h.languageM.List(ctx)
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

func (h *LanguageHandler) Add(ctx *gin.Context) {
	var params countrydto.LanguageParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.Language
	copier.Copy(&data, params)
	err := h.languageM.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *LanguageHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{}) {
		return
	}

	type LanguageIDs struct {
		ID int64 `json:"id" binding:"required"`
	}
	var params = struct {
		LanguageIDs
		countrydto.LanguageParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.languageM.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.languageM.Edit(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *LanguageHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int64 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		FailByErr(ctx, validator.GetError(param, err))
		return
	}
	err := h.languageM.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	SuccessWithMessage(ctx, "Deleted successfully")
}
