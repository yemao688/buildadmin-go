package handler

import (
	adminModel "go-build-admin/app/admin/model"
	"go-build-admin/app/common/model"
	"go-build-admin/app/pkg/header"
	"go-build-admin/utils"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AjaxHandler struct {
	log          *zap.Logger
	areaM        *model.AreaModel
	tableM       *adminModel.TableModel
	uploadHelper *model.UploadHelper
}

func NewAjaxHandler(log *zap.Logger, areaM *model.AreaModel, tableM *adminModel.TableModel, uploadHelper *model.UploadHelper) *AjaxHandler {
	return &AjaxHandler{log: log, areaM: areaM, tableM: tableM, uploadHelper: uploadHelper}
}

func (h *AjaxHandler) Upload(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	adminAuth := header.GetAdminAuth(ctx)

	h.uploadHelper.SetFile(file)
	result, err := h.uploadHelper.Upload(ctx, adminAuth.Id, 0)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"file": result,
	})
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
	suffix := ctx.Request.FormValue("suffix")
	if suffix == "" {
		suffix = "file"
	}
	background := ctx.Request.FormValue("background")

	svgBytes := []byte(utils.BuildSuffixSvg(suffix, background))
	ctx.Header("Content-Length", strconv.Itoa(len(svgBytes)))
	ctx.Header("Content-Type", "image/svg+xml")
	ctx.Data(http.StatusOK, "image/jpeg", svgBytes)
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
