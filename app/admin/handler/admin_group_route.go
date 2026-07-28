// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type AdminGroupRegistrar struct {
	handler *AdminGroupHandler
}

func NewAdminGroupRegistrar(handler *AdminGroupHandler) *AdminGroupRegistrar {
	return &AdminGroupRegistrar{handler: handler}
}

const adminGroupRoute = "auth.Group"

func (r *AdminGroupRegistrar) Group() string { return "admin" }

func (r *AdminGroupRegistrar) Register(g gin.IRoutes) {
	g.GET(adminGroupRoute+"/index", r.handler.Index)
	g.POST(adminGroupRoute+"/add", r.handler.Add)
	g.GET(adminGroupRoute+"/edit", r.handler.One)
	g.POST(adminGroupRoute+"/edit", r.handler.Edit)
	g.DELETE(adminGroupRoute+"/del", r.handler.Del)
}

func (r *AdminGroupRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(adminGroupRoute)
}
