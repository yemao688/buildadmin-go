// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type DashboardRegistrar struct {
	handler *DashboardHandler
}

func NewDashboardRegistrar(handler *DashboardHandler) *DashboardRegistrar {
	return &DashboardRegistrar{handler: handler}
}

func (r *DashboardRegistrar) Group() string { return "admin" }

func (r *DashboardRegistrar) Register(g gin.IRoutes) {
	g.GET("Dashboard/index", r.handler.Index)
}

func (r *DashboardRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
