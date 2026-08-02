// 由 RouteRegistrar 模式维护（手写模块）
package auth

import (
	adminhandler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type AdminRegistrar struct {
	handler *AdminHandler
}

func NewAdminRegistrar(handler *AdminHandler) *AdminRegistrar {
	return &AdminRegistrar{handler: handler}
}

const adminRoute = "auth.Admin"

func (r *AdminRegistrar) Group() string { return "admin" }

func (r *AdminRegistrar) Register(g gin.IRoutes) {
	g.GET(adminRoute+"/index", r.handler.Index)
	g.POST(adminRoute+"/add", r.handler.Add)
	g.GET(adminRoute+"/edit", r.handler.One)
	g.POST(adminRoute+"/edit", r.handler.Edit)
	g.DELETE(adminRoute+"/del", r.handler.Del)
}

func (r *AdminRegistrar) Capabilities() []middleware.AtomicRoute {
	return adminhandler.CRUDCapabilities(adminRoute)
}
