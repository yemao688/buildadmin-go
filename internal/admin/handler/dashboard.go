package handler

import (
	adminauth "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DashboardHandler struct {
	log        *zap.Logger
	adminRuleM *adminauth.AdminRuleRepository
}

func NewDashboardHandler(log *zap.Logger, adminRuleM *adminauth.AdminRuleRepository) *DashboardHandler {
	return &DashboardHandler{
		log:        log,
		adminRuleM: adminRuleM,
	}
}

func (h *DashboardHandler) Index(ctx *gin.Context) {
	response.Success(ctx, map[string]string{
		"remark": remark(ctx, h.adminRuleM),
	})
}
