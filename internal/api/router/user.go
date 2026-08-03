// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	api "buildadmin-go/internal/api/handler"

	"github.com/gin-gonic/gin"
)

type UserRegistrar struct {
	handler *api.UserHandler
}

func NewUserRegistrar(handler *api.UserHandler) *UserRegistrar {
	return &UserRegistrar{handler: handler}
}

func (r *UserRegistrar) Register(g gin.IRoutes) {
	g.POST("user/login", r.handler.Login)
	g.POST("user/register", r.handler.Register)
	g.POST("user/logout", r.handler.Logout)
}
