// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type UserGroupRegistrar struct {
	handler *UserGroupHandler
}

func NewUserGroupRegistrar(handler *UserGroupHandler) *UserGroupRegistrar {
	return &UserGroupRegistrar{handler: handler}
}

const userGroupRoute = "user.Group"

func (r *UserGroupRegistrar) Group() string { return "admin" }

func (r *UserGroupRegistrar) Register(g gin.IRoutes) {
	g.GET(userGroupRoute+"/index", r.handler.Index)
	g.POST(userGroupRoute+"/add", r.handler.Add)
	g.GET(userGroupRoute+"/edit", r.handler.One)
	g.POST(userGroupRoute+"/edit", r.handler.Edit)
	g.DELETE(userGroupRoute+"/del", r.handler.Del)
}

func (r *UserGroupRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
