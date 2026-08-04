// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	adminmiddleware "buildadmin-go/internal/admin/middleware"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type ModuleRegistrar struct {
	handler *handler.ModuleHandler
}

func NewModuleRegistrar(handler *handler.ModuleHandler) *ModuleRegistrar {
	return &ModuleRegistrar{handler: handler}
}

const moduleRoute = "module"

func (r *ModuleRegistrar) Group() string { return "admin" }

func (r *ModuleRegistrar) Register(g gin.IRoutes) {
	adminmiddleware.RegisterPermissionExempt("module", "state", "dependentinstallcomplete")
	g.GET(moduleRoute+"/index", r.handler.Index)
}

func (r *ModuleRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
