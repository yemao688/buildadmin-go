// Package router 注册 user_money_log（会员余额日志）模块路由，由 RouteRegistrar 模式维护（手写模块）。
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type UserLogRegistrar struct {
	userHandler     *handler.UserHandler
	moneyLogHandler *handler.MoneyLogHandler
}

func NewUserLogRegistrar(
	userHandler *handler.UserHandler,
	moneyLogHandler *handler.MoneyLogHandler,
) *UserLogRegistrar {
	return &UserLogRegistrar{
		userHandler:     userHandler,
		moneyLogHandler: moneyLogHandler,
	}
}

func (r *UserLogRegistrar) Register(g gin.IRoutes) {
	g.GET("user.MoneyLog/index", r.moneyLogHandler.Index)
	g.GET("user.MoneyLog/add", r.userHandler.One)
	g.POST("user.MoneyLog/add", r.moneyLogHandler.Add)
}

func (r *UserLogRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
