package handler

import (
	adminModel "go-build-admin/app/admin/model"
	"go-build-admin/app/common/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AjaxHandler struct {
	log    *zap.Logger
	areaM  *model.AreaModel
	tableM *adminModel.TableModel
}

func NewAjaxHandler(log *zap.Logger, areaM *model.AreaModel, tableM *adminModel.TableModel) *AjaxHandler {
	return &AjaxHandler{log: log, areaM: areaM, tableM: tableM}
}

func (h *AjaxHandler) Upload(ctx *gin.Context) {

	Success(ctx, "")
}

// 省份地区数据
func (h *AjaxHandler) Area(ctx *gin.Context) {
	result, err := h.areaM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *AjaxHandler) BuildSuffixSvg(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *AjaxHandler) GetTablePk(ctx *gin.Context) {
	table := ctx.Request.FormValue("table")
	pk := h.tableM.GetTablePk(table)
	Success(ctx, map[string]string{
		"pk": pk,
	})
}

func (h *AjaxHandler) GetTableFieldList(ctx *gin.Context) {
	table := ctx.Request.FormValue("table")
	pk := h.tableM.GetTablePk(table)

	Success(ctx, map[string]any{
		"pk":        pk,
		"fieldList": h.tableM.GetTableFields(table, true),
	})
}

func (h *AjaxHandler) ChangeTerminalConfig(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *AjaxHandler) ClearCache(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *AjaxHandler) Terminal(ctx *gin.Context) {

	Success(ctx, "")
}
