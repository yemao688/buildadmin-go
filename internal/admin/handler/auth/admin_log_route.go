// 由 RouteRegistrar 模式维护（手写模块）
package auth

import (
	"go-build-admin/internal/middleware"
	"net/http"

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

func (r *AdminLogRegistrar) Capabilities() []middleware.AtomicRoute {
	// AdminLog 仅有 del 写操作；声明 add/edit 会导致与注册路由不匹配
	return []middleware.AtomicRoute{{Route: "auth/adminlog", Action: "del", Method: http.MethodDelete}}
}
