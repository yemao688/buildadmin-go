// 由 RouteRegistrar 模式维护（手写模块）
package user

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type RuleRegistrar struct {
	handler *RuleHandler
}

func NewRuleRegistrar(handler *RuleHandler) *RuleRegistrar {
	return &RuleRegistrar{handler: handler}
}

const ruleRoute = "user.Rule"

func (r *RuleRegistrar) Group() string { return "admin" }

func (r *RuleRegistrar) Register(g gin.IRoutes) {
	CRUDRoutes(g, ruleRoute, r.handler)
}

func (r *RuleRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
