package country

import (
	adminhandler "go-build-admin/internal/admin/handler"
	countrymodel "go-build-admin/internal/admin/model/country"
	"go-build-admin/internal/admin/validate"
	model "go-build-admin/internal/model"
	"go-build-admin/internal/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type LanguageContentHandler struct {
	adminhandler.Base
	log              *zap.Logger
	languageContentM *countrymodel.LanguageContentModel
}

func NewLanguageContentHandler(log *zap.Logger, languageContentM *countrymodel.LanguageContentModel) *LanguageContentHandler {
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

type LanguageContentParam struct {
	Lan   string `json:"lan"`   // 语言代码
	Group string `json:"group"` // 分组
	Key   string `json:"key"`   // 键
	Type  string `json:"type"`  // 类型:0=文本,1=富文本,2=图片
	Value string `json:"value"` // 值
}

func (h *LanguageContentHandler) Add(ctx *gin.Context) {
	var params LanguageContentParam
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
		LanguageContentParam
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
