package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type IndexHandler struct {
	log *zap.Logger
}

func NewIndexHandler(log *zap.Logger) *IndexHandler {
	return &IndexHandler{log: log}
}

// 前台和会员中心的初始化请求
func (h *IndexHandler) Index(ctx *gin.Context) {

	Success(ctx, "")
}
