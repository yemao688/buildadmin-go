package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AdminLogHandler struct {
	log       *zap.Logger
	adminLogM *model.AdminLogModel
}

func NewAdminLogHandler(log *zap.Logger, adminLogM *model.AdminLogModel) *AdminLogHandler {
	return &AdminLogHandler{log: log, adminLogM: adminLogM}
}

func (h *AdminLogHandler) Index(ctx *gin.Context) {
	result, err := h.adminLogM.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}
