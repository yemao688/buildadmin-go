package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AdminLogHandler struct {
	Base
	log       *zap.Logger
	adminLogM *model.AdminLogModel
}

func NewAdminLogHandler(log *zap.Logger, adminLogM *model.AdminLogModel) *AdminLogHandler {
	return &AdminLogHandler{
		Base: Base{currentM: adminLogM},
		log:  log, adminLogM: adminLogM}
}

func (h *AdminLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	result, total, err := h.adminLogM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":   result,
		"total":  total,
		"remark": "",
	})
}
