// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type AccountRegistrar struct {
	handler *AccountHandler
}

func NewAccountRegistrar(handler *AccountHandler) *AccountRegistrar {
	return &AccountRegistrar{handler: handler}
}

func (r *AccountRegistrar) Group() string { return "api" }

func (r *AccountRegistrar) Register(g gin.IRoutes) {
	g.POST("account/retrievePassword", r.handler.RetrievePassword)

	g.GET("account/overview", r.handler.Overview)
	g.GET("account/profile", r.handler.Profile)
	g.POST("account/profile", r.handler.Profile)
	g.POST("account/verification", r.handler.Verification)
	g.POST("account/changeBind", r.handler.ChangeBind)
	g.POST("account/changePassword", r.handler.ChangePassword)
	g.GET("account/integral", r.handler.Integral)
	g.GET("account/balance", r.handler.Balance)
}

func (r *AccountRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
