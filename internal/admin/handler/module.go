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

// NoNeedPermissionActions 声明需登录但免权限的 action。index 是模块商店
// 首屏占位接口（超管菜单可见，返回空列表），与文档声明的显式豁免一致；
// 其余 action 为 PHP 上游兼容的桩占位。
func (h *ModuleHandler) NoNeedPermissionActions() []string {
	return []string{"index", "state", "dependentinstallcomplete"}
}
