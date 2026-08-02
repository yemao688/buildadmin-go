// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/internal/middleware"

	"github.com/gin-gonic/gin"
)

type ModuleRegistrar struct {
	handler *ModuleHandler
}

func NewModuleRegistrar(handler *ModuleHandler) *ModuleRegistrar {
	return &ModuleRegistrar{handler: handler}
}

const moduleRoute = "module"

func (r *ModuleRegistrar) Group() string { return "admin" }

func (r *ModuleRegistrar) Register(g gin.IRoutes) {
	middleware.RegisterPermissionExempt("module", "index", "state", "dependentinstallcomplete")
	g.GET(moduleRoute+"/index", r.handler.Index)
}

func (r *ModuleRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
