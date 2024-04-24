package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DashboardHandler struct {
	log        *zap.Logger
	adminRuleM *model.AdminRuleModel
}

func DewdashboardHandler(log *zap.Logger, adminRuleM *model.AdminRuleModel) *DashboardHandler {
	return &DashboardHandler{log: log, adminRuleM: adminRuleM}
}

func (h *DashboardHandler) Dashboard(ctx *gin.Context) {
	remark := h.adminRuleM.GetRemark(ctx)
	Success(ctx, map[string]string{
		"remark": remark,
	})
}
