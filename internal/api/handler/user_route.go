// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/internal/middleware"

	"github.com/gin-gonic/gin"
)

type UserRegistrar struct {
	handler *UserHandler
}

func NewUserRegistrar(handler *UserHandler) *UserRegistrar {
	return &UserRegistrar{handler: handler}
}

func (r *UserRegistrar) Group() string { return "api" }

func (r *UserRegistrar) Register(g gin.IRoutes) {
	g.POST("user/login", r.handler.Login)
	g.POST("user/register", r.handler.Register)
	g.POST("user/logout", r.handler.Logout)
}

func (r *UserRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
