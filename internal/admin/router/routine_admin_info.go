// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type AdminInfoRegistrar struct {
	handler *handler.AdminInfoHandler
}

func NewAdminInfoRegistrar(handler *handler.AdminInfoHandler) *AdminInfoRegistrar {
	return &AdminInfoRegistrar{handler: handler}
}

const adminInfoRoute = "routine.AdminInfo"

func (r *AdminInfoRegistrar) Group() string { return "admin" }

func (r *AdminInfoRegistrar) Register(g gin.IRoutes) {
	g.GET(adminInfoRoute+"/index", r.handler.Index)
	g.POST(adminInfoRoute+"/edit", r.handler.Edit)
}

func (r *AdminInfoRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
