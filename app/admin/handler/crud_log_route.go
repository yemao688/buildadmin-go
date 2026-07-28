// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type CrudLogRegistrar struct {
	handler *CrudLogHandler
}

func NewCrudLogRegistrar(handler *CrudLogHandler) *CrudLogRegistrar {
	return &CrudLogRegistrar{handler: handler}
}

const crudLogRoute = "crud.Log"

func (r *CrudLogRegistrar) Group() string { return "admin" }

func (r *CrudLogRegistrar) Register(g gin.IRoutes) {
	g.GET(crudLogRoute+"/index", r.handler.Index)
}

func (r *CrudLogRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
