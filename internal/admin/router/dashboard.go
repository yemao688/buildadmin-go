// Package router 注册 dashboard（仪表盘）模块路由，由 RouteRegistrar 模式维护（手写模块）。
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type DashboardRegistrar struct {
	handler *handler.DashboardHandler
}

func NewDashboardRegistrar(handler *handler.DashboardHandler) *DashboardRegistrar {
	return &DashboardRegistrar{handler: handler}
}

func (r *DashboardRegistrar) Register(g gin.IRoutes) {
	g.GET("Dashboard/index", r.handler.Index)
}

func (r *DashboardRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
