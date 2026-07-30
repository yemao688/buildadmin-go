// 由 RouteRegistrar 模式维护（手写模块）
package user

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type UserLogRegistrar struct {
	userHandler     *UserHandler
	moneyLogHandler *UserMoneyLogHandler
	scoreLogHandler *UserScoreLogHandler
}

func NewUserLogRegistrar(
	userHandler *UserHandler,
	moneyLogHandler *UserMoneyLogHandler,
	scoreLogHandler *UserScoreLogHandler,
) *UserLogRegistrar {
	return &UserLogRegistrar{
		userHandler:     userHandler,
		moneyLogHandler: moneyLogHandler,
		scoreLogHandler: scoreLogHandler,
	}
}

func (r *UserLogRegistrar) Group() string { return "admin" }

func (r *UserLogRegistrar) Register(g gin.IRoutes) {
	g.GET("user.MoneyLog/index", r.moneyLogHandler.Index)
	g.GET("user.MoneyLog/add", r.userHandler.One)
	g.POST("user.MoneyLog/add", r.moneyLogHandler.Add)

	g.GET("user.ScoreLog/index", r.scoreLogHandler.Index)
	g.GET("user.ScoreLog/add", r.userHandler.One)
	g.POST("user.ScoreLog/add", r.scoreLogHandler.Add)
}

func (r *UserLogRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
