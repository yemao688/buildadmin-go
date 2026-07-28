// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

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
	g.GET(moduleRoute+"/index", r.handler.Index)
	g.POST(moduleRoute+"/uploadCompleted", r.handler.UploadCompleted)
}

func (r *ModuleRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
