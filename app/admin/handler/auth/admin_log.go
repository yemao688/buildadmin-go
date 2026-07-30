package auth

import (
	adminmodel "go-build-admin/app/admin/model/auth"
	"go-build-admin/app/admin/validate"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AdminLogHandler struct {
	Base
	log       *zap.Logger
	adminLogM *adminmodel.AdminLogModel
}

func NewAdminLogHandler(log *zap.Logger, adminLogM *adminmodel.AdminLogModel) *AdminLogHandler {
	return &AdminLogHandler{
		Base: NewBase(adminLogM),
		log:  log, adminLogM: adminLogM}
}

func (h *AdminLogHandler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
		return
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

func (h *AdminLogHandler) Del(ctx *gin.Context) {
	var params validate.Ids
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validate.GetError(params, err))
		return
	}
	if err := h.adminLogM.Del(ctx, params.Ids); err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
