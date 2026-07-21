package handler

import (
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
	uploadHelper *model.UploadHelper
}

func NewAjaxHandler(log *zap.Logger, areaM *model.AreaModel, uploadHelper *model.UploadHelper) *AjaxHandler {
	return &AjaxHandler{log: log, areaM: areaM, uploadHelper: uploadHelper}
}

func (h *AjaxHandler) Upload(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	userAuth := header.GetUserAuth(ctx)

	h.uploadHelper.SetFile(file)
	result, err := h.uploadHelper.Upload(ctx, 0, userAuth.Id)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"file": result,
	})
}

func (h *AjaxHandler) AliossCallback(ctx *gin.Context) {
	var params model.OSSCallback
	if err := ctx.ShouldBind(&params); err != nil {
		FailByErr(ctx, err)
		return
	}
	auth := header.GetUserAuth(ctx)
	result, err := h.uploadHelper.CompleteOSS(params, 0, auth.Id)
	if err != nil {
		h.log.Error("AliOSS callback failed", zap.Error(err))
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{"file": result})
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
