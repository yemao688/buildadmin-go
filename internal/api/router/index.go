// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	api "buildadmin-go/internal/api/handler"

	"github.com/gin-gonic/gin"
)

type IndexRegistrar struct {
	handler *api.IndexHandler
}

func NewIndexRegistrar(handler *api.IndexHandler) *IndexRegistrar {
	return &IndexRegistrar{handler: handler}
}

func (r *IndexRegistrar) Register(g gin.IRoutes) {
	g.GET("index/index", r.handler.Index)
}
