// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type UserRegistrar struct {
	handler *handler.UserHandler
}

func NewUserRegistrar(handler *handler.UserHandler) *UserRegistrar {
	return &UserRegistrar{handler: handler}
}

const userRoute = "user.User"

func (r *UserRegistrar) Group() string { return "admin" }

func (r *UserRegistrar) Register(g gin.IRoutes) {
	g.GET(userRoute+"/index", r.handler.Index)
	g.POST(userRoute+"/add", r.handler.Add)
	g.GET(userRoute+"/edit", r.handler.One)
	g.POST(userRoute+"/edit", r.handler.Edit)
	g.DELETE(userRoute+"/del", r.handler.Del)
}

func (r *UserRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(userRoute)
}
