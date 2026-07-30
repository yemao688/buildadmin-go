// 由 RouteRegistrar 模式维护（手写模块）
package user

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type UserRuleRegistrar struct {
	handler *UserRuleHandler
}

func NewUserRuleRegistrar(handler *UserRuleHandler) *UserRuleRegistrar {
	return &UserRuleRegistrar{handler: handler}
}

const userRuleRoute = "user.Rule"

func (r *UserRuleRegistrar) Group() string { return "admin" }

func (r *UserRuleRegistrar) Register(g gin.IRoutes) {
	CRUDRoutes(g, userRuleRoute, r.handler)
}

func (r *UserRuleRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
