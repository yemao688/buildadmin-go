package handler

import (
	"go-build-admin/app/admin/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AdminHandler struct {
	log    *zap.Logger
	adminM *model.AdminModel
}

func NewAdminHandler(log *zap.Logger, adminM *model.AdminModel) *AdminHandler {
	return &AdminHandler{log: log, adminM: adminM}
}

func (h *AdminHandler) Index(ctx *gin.Context) {
	result, err := h.adminM.List(ctx, 1)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *AdminHandler) Add(ctx *gin.Context) {
	result, err := h.adminM.List(ctx, 1)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *AdminHandler) Edit(ctx *gin.Context) {
	result, err := h.adminM.List(ctx, 1)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

func (h *AdminHandler) Del(ctx *gin.Context) {
	result, err := h.adminM.List(ctx, 1)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}

/**
 * 检查分组权限
 * @throws Throwable
 */
func (h *AdminHandler) CheckGroupAuth(ctx *gin.Context) {
	result, err := h.adminM.List(ctx, 1)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, result)
}
