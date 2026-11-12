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
	adminmiddleware.RegisterHandlerExemptions("module", r.handler)
	g.GET(moduleRoute+"/index", r.handler.Index)
	// 模块商店的其余动作是 PHP 上游兼容桩：handler 返回空成功，但路由必须
	// 注册，否则前端从商店列表点安装/卸载会 404。真正的模块安装能力不在
	// Go 框架范围内，这里只保证契约完整。
	g.GET(moduleRoute+"/state", r.handler.State)
	g.POST(moduleRoute+"/install", r.handler.Install)
	g.POST(moduleRoute+"/dependentInstallComplete", r.handler.DependentInstallComplete)
	g.POST(moduleRoute+"/changeState", r.handler.ChangeState)
	g.POST(moduleRoute+"/uninstall", r.handler.Uninstall)
	g.POST(moduleRoute+"/update", r.handler.Update)
	g.POST(moduleRoute+"/upload", r.handler.Upload)
}

func (r *ModuleRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
