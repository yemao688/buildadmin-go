// Package router 注册 admin_rule（权限规则）模块路由，由 RouteRegistrar 模式维护（手写模块）。
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

func (r *AdminRuleRegistrar) Register(g gin.IRoutes) {
	CRUDRoutes(g, adminRuleRoute, r.handler)
}

func (r *AdminRuleRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(adminRuleRoute)
}
