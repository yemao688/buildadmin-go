package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AjaxHandler struct {
	log *zap.Logger
}

func NewAjaxHandler(log *zap.Logger) *AjaxHandler {
	return &AjaxHandler{log: log}
}

func (h *AjaxHandler) Upload(ctx *gin.Context) {

	Success(ctx, "")
}

// 省份地区数据
func (h *AjaxHandler) Area(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *AjaxHandler) BuildSuffixSvg(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *AjaxHandler) GetTablePk(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *AjaxHandler) GetTableFieldList(ctx *gin.Context) {

	Success(ctx, "")
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
