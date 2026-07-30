// 由 RouteRegistrar 模式维护（手写模块）
package auth

import (
	adminhandler "go-build-admin/app/admin/handler"
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type AdminRuleRegistrar struct {
	handler *AdminRuleHandler
}

func NewAdminRuleRegistrar(handler *AdminRuleHandler) *AdminRuleRegistrar {
	return &AdminRuleRegistrar{handler: handler}
}

const adminRuleRoute = "auth.Rule"

func (r *AdminRuleRegistrar) Group() string { return "admin" }

func (r *AdminRuleRegistrar) Register(g gin.IRoutes) {
	adminhandler.CRUDRoutes(g, adminRuleRoute, r.handler)
}

func (r *AdminRuleRegistrar) Capabilities() []middleware.AtomicRoute {
	return adminhandler.CRUDCapabilities(adminRuleRoute)
}
