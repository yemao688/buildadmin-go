// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type DemoRegistrar struct {
	handler *DemoHandler
}

func NewDemoRegistrar(handler *DemoHandler) *DemoRegistrar {
	return &DemoRegistrar{handler: handler}
}

func (r *DemoRegistrar) Group() string { return "api" }

func (r *DemoRegistrar) Register(g gin.IRoutes) {
	g.POST("demo/index", r.handler.Index)
}

func (r *DemoRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
