// 由 RouteRegistrar 模式维护（手写模块）
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type SensitiveDataRegistrar struct {
	handler *SensitiveDataHandler
}

func NewSensitiveDataRegistrar(handler *SensitiveDataHandler) *SensitiveDataRegistrar {
	return &SensitiveDataRegistrar{handler: handler}
}

const sensitiveDataRoute = "security.SensitiveData"

func (r *SensitiveDataRegistrar) Group() string { return "admin" }

func (r *SensitiveDataRegistrar) Register(g gin.IRoutes) {
	g.GET(sensitiveDataRoute+"/index", r.handler.Index)
	g.GET(sensitiveDataRoute+"/add", r.handler.Add)
	g.POST(sensitiveDataRoute+"/add", r.handler.Add)
	g.GET(sensitiveDataRoute+"/edit", r.handler.One)
	g.POST(sensitiveDataRoute+"/edit", r.handler.Edit)
	g.DELETE(sensitiveDataRoute+"/del", r.handler.Del)
}

func (r *SensitiveDataRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(sensitiveDataRoute)
}
