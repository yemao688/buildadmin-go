package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DemoHandler struct {
	log *zap.Logger
}

func NewDemoHandler(log *zap.Logger) *DemoHandler {
	return &DemoHandler{log: log}
}

// 前台和会员中心的初始化请求
func (h *DemoHandler) Index(ctx *gin.Context) {

	Success(ctx, map[string]any{})
}
