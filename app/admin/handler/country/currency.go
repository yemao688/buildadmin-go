package country

import (
	adminhandler "go-build-admin/app/admin/handler"
	model "go-build-admin/app/admin/model/country"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type CurrencyHandler struct {
	adminhandler.Base
	log       *zap.Logger
	currencyM *model.CurrencyModel
}

func NewCurrencyHandler(log *zap.Logger, currencyM *model.CurrencyModel) *CurrencyHandler {
	return &CurrencyHandler{Base: adminhandler.NewBase(currencyM), log: log, currencyM: currencyM}
}

func (h *CurrencyHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		adminhandler.Success(ctx, data)
	}
	list, total, err := h.currencyM.List(ctx)
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

type CurrencyParam struct {
	Code   string               `json:"code"`   // 货币代码
	Name   string               `json:"name"`   // 货币名称
	Symbol string               `json:"symbol"` // 货币符号
	Rate   validate.FlexFloat64 `json:"rate"`   // 汇率
	Status validate.FlexInt32   `json:"status"` // 状态:0=禁用,1=启用
	Weigh  validate.FlexInt32   `json:"weigh"`  // 权重
}

func (h *CurrencyHandler) Add(ctx *gin.Context) {
	var params CurrencyParam
	if err := ctx.ShouldBindJSON(&params); err != nil {
		adminhandler.FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.Currency
	copier.Copy(&data, params)
	err := h.currencyM.Add(ctx, data)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *CurrencyHandler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{}) {
		return
	}

	type CurrencyIDs struct {
		ID int64 `json:"id" binding:"required"`
	}
	var params = struct {
		CurrencyIDs
		CurrencyParam
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		adminhandler.FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.currencyM.GetOne(ctx, params.ID)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.currencyM.Edit(ctx, data)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.Success(ctx, "")
}

func (h *CurrencyHandler) Del(ctx *gin.Context) {
	var param struct {
		Ids []int64 `form:"ids[]" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&param); err != nil {
		adminhandler.FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.currencyM.Del(ctx, param.Ids)
	if err != nil {
		adminhandler.FailByErr(ctx, err)
		return
	}
	adminhandler.SuccessWithMessage(ctx, "Deleted successfully")
}
