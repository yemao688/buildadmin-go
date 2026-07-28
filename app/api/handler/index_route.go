// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type IndexRegistrar struct {
	handler *IndexHandler
}

func NewIndexRegistrar(handler *IndexHandler) *IndexRegistrar {
	return &IndexRegistrar{handler: handler}
}

func (r *IndexRegistrar) Group() string { return "api" }

func (r *IndexRegistrar) Register(g gin.IRoutes) {
	g.GET("index/index", r.handler.Index)
}

func (r *IndexRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
