package handler

import (
	adminauth "buildadmin-go/internal/admin/repository"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DashboardHandler struct {
	Base
	log        *zap.Logger
	adminRuleM *adminauth.AdminRuleRepository
}

func NewDashboardHandler(log *zap.Logger, adminRuleM *adminauth.AdminRuleRepository) *DashboardHandler {
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
