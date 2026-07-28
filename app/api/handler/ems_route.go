// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type EmsRegistrar struct {
	handler *EmsHandler
}

func NewEmsRegistrar(handler *EmsHandler) *EmsRegistrar {
	return &EmsRegistrar{handler: handler}
}

func (r *EmsRegistrar) Group() string { return "api" }

func (r *EmsRegistrar) Register(g gin.IRoutes) {
	g.POST("Ems/send", r.handler.Send)
}

func (r *EmsRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
