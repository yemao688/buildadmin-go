package country

import (
	countrydto "buildadmin-go/internal/admin/dto/country"
	adminhandler "buildadmin-go/internal/admin/handler"
	countrymodel "buildadmin-go/internal/admin/repository/country"
	"buildadmin-go/internal/admin/validate"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type LanguageContentHandler struct {
	adminhandler.Base
	log              *zap.Logger
	languageContentM *countrymodel.LanguageContentRepository
}

func NewLanguageContentHandler(log *zap.Logger, languageContentM *countrymodel.LanguageContentRepository) *LanguageContentHandler {
	return &LanguageContentHandler{Base: adminhandler.NewBase(languageContentM), log: log, languageContentM: languageContentM}
}

func (h *LanguageContentHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		adminhandler.Success(ctx, data)
	}
	list, total, err := h.languageContentM.List(ctx)
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

func (h *LanguageContentHandler) Add(ctx *gin.Context) {
	var params countrydto.LanguageContentParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		adminhandler.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.LanguageContent
	copier.Copy(&data, params)
	err := h.languageContentM.Add(ctx, data)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *LanguageContentHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{}) {
		return
	}

	type LanguageContentIDs struct {
		ID int64 `json:"id" binding:"required"`
	}
	var params = struct {
		LanguageContentIDs
		countrydto.LanguageContentParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		adminhandler.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.languageContentM.GetOne(ctx, params.ID)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.languageContentM.Edit(ctx, data)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *LanguageContentHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int64 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.languageContentM.Del(ctx, param.Ids)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.SuccessWithMessage(ctx, "Deleted successfully")
}
