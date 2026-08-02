package handler

import (
	adminauth "go-build-admin/internal/admin/model/auth"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DashboardHandler struct {
	Base
	log        *zap.Logger
	adminRuleM *adminauth.AdminRuleModel
}

func NewDashboardHandler(log *zap.Logger, adminRuleM *adminauth.AdminRuleModel) *DashboardHandler {
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
