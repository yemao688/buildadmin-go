package country

import (
	countrydto "go-build-admin/internal/admin/dto/country"
	adminhandler "go-build-admin/internal/admin/handler"
	countrymodel "go-build-admin/internal/admin/repository/country"
	"go-build-admin/internal/admin/validate"
	model "go-build-admin/internal/model"
	"go-build-admin/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type LanguageHandler struct {
	adminhandler.Base
	log       *zap.Logger
	languageM *countrymodel.LanguageRepository
}

func NewLanguageHandler(log *zap.Logger, languageM *countrymodel.LanguageRepository) *LanguageHandler {
	return &LanguageHandler{Base: adminhandler.NewBase(languageM), log: log, languageM: languageM}
}

func (h *LanguageHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		adminhandler.Success(ctx, data)
	}
	list, total, err := h.languageM.List(ctx)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, map[string]any{
		"list":   list,
		"total":  total,
		"remark": "",
	})
}

func (h *LanguageHandler) Add(ctx *gin.Context) {
	var params countrydto.LanguageParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		adminhandler.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.Language
	copier.Copy(&data, params)
	err := h.languageM.Add(ctx, data)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
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
		adminhandler.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.languageM.GetOne(ctx, params.ID)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.languageM.Edit(ctx, data)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *LanguageHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int64 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.languageM.Del(ctx, param.Ids)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.SuccessWithMessage(ctx, "Deleted successfully")
}
