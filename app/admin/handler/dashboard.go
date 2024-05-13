package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DashboardHandler struct {
	log      *zap.Logger
	currentM *model.AdminRuleModel
}

func NewDashboardHandler(log *zap.Logger, currentM *model.AdminRuleModel) *DashboardHandler {
	return &DashboardHandler{log: log, currentM: currentM}
}

func (h *DashboardHandler) Index(ctx *gin.Context) {
	remark := h.currentM.GetRemark(ctx)
	Success(ctx, map[string]string{
		"remark": remark,
	})
}
