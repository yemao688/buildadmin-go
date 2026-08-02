// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type AdminRuleRegistrar struct {
	handler *handler.AdminRuleHandler
}

func NewAdminRuleRegistrar(handler *handler.AdminRuleHandler) *AdminRuleRegistrar {
	return &AdminRuleRegistrar{handler: handler}
}

const adminRuleRoute = "auth.Rule"

func (r *AdminRuleRegistrar) Group() string { return "admin" }

func (r *AdminRuleRegistrar) Register(g gin.IRoutes) {
	handler.CRUDRoutes(g, adminRuleRoute, r.handler)
}

func (r *AdminRuleRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(adminRuleRoute)
}
