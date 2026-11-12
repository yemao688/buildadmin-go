package handler

import (
	"buildadmin-go/internal/pkg/response"
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

	response.Success(ctx, map[string]any{
		"list":  []any{},
		"total": 0,
	})
}

func (h *ModuleHandler) State(ctx *gin.Context) {

	response.Success(ctx, "")
}

func (h *ModuleHandler) Install(ctx *gin.Context) {

	response.Success(ctx, "")
}

func (h *ModuleHandler) DependentInstallComplete(ctx *gin.Context) {

	response.Success(ctx, "")
}

func (h *ModuleHandler) ChangeState(ctx *gin.Context) {

	response.Success(ctx, "")
}

func (h *ModuleHandler) Uninstall(ctx *gin.Context) {

	response.Success(ctx, "")
}

func (h *ModuleHandler) Update(ctx *gin.Context) {

	response.Success(ctx, "")
}

func (h *ModuleHandler) Upload(ctx *gin.Context) {

	response.Success(ctx, "")
}

// NoNeedPermissionActions 声明需登录但免权限的 action。模块商店的全部动作
// 都是 PHP 上游兼容的桩占位（Go 框架没有模块安装能力），返回成功但不执行
// 任何安装；它们统一免 admin_rule，避免前端从商店列表点击时 403/404。
func (h *ModuleHandler) NoNeedPermissionActions() []string {
	return []string{"index", "state", "dependentinstallcomplete", "install", "uninstall", "changestate", "update", "upload"}
}
