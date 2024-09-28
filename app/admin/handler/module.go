package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type ModuleHandler struct {
	log *zap.Logger
}

func NewModuleHandler(log *zap.Logger) *ModuleHandler {
	return &ModuleHandler{
		log: log,
	}
}

func (h *ModuleHandler) Index(ctx *gin.Context) {

	SuccessMsg(ctx, nil, "待实现")
}

func (h *ModuleHandler) State(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *ModuleHandler) Install(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *ModuleHandler) DependentInstallComplete(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *ModuleHandler) ChangeState(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *ModuleHandler) Uninstall(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *ModuleHandler) Update(ctx *gin.Context) {

	Success(ctx, "")
}

func (h *ModuleHandler) Upload(ctx *gin.Context) {

	Success(ctx, "")
}
