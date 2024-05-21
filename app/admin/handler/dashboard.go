package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DashboardHandler struct {
	Base
	log        *zap.Logger
	adminRuleM *model.AdminRuleModel
}

func NewDashboardHandler(log *zap.Logger, adminRuleM *model.AdminRuleModel) *DashboardHandler {
	return &DashboardHandler{
		Base:       Base{currentM: adminRuleM},
		log:        log,
		adminRuleM: adminRuleM,
	}
}

func (h *DashboardHandler) Index(ctx *gin.Context) {
	remark := h.GetRemark(ctx)
	Success(ctx, map[string]string{
		"remark": remark,
	})
}
