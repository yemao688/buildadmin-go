// 由 RouteRegistrar 模式维护（手写模块）
package user

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type GroupRegistrar struct {
	handler *GroupHandler
}

func NewGroupRegistrar(handler *GroupHandler) *GroupRegistrar {
	return &GroupRegistrar{handler: handler}
}

const groupRoute = "user.Group"

func (r *GroupRegistrar) Group() string { return "admin" }

func (r *GroupRegistrar) Register(g gin.IRoutes) {
	g.GET(groupRoute+"/index", r.handler.Index)
	g.POST(groupRoute+"/add", r.handler.Add)
	g.GET(groupRoute+"/edit", r.handler.One)
	g.POST(groupRoute+"/edit", r.handler.Edit)
	g.DELETE(groupRoute+"/del", r.handler.Del)
}

func (r *GroupRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
