// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type AdminLogRegistrar struct {
	handler *AdminLogHandler
}

func NewAdminLogRegistrar(handler *AdminLogHandler) *AdminLogRegistrar {
	return &AdminLogRegistrar{handler: handler}
}

func (r *AdminLogRegistrar) Group() string { return "admin" }

func (r *AdminLogRegistrar) Register(g gin.IRoutes) {
	g.GET("auth.AdminLog/index", r.handler.Index)
	g.DELETE("auth.AdminLog/del", r.handler.Del)
}

func (r *AdminLogRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
