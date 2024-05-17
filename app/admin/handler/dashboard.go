package handler

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"

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
		"remark": utils.Lang(ctx, remark, nil),
	})
}
