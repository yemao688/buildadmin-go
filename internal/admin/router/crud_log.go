// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	adminmiddleware "buildadmin-go/internal/admin/middleware"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type LogRegistrar struct {
	handler *handler.LogHandler
}

func NewLogRegistrar(handler *handler.LogHandler) *LogRegistrar {
	return &LogRegistrar{handler: handler}
}

const logRoute = "crud.Log"

func (r *LogRegistrar) Group() string { return "admin" }

func (r *LogRegistrar) Register(g gin.IRoutes) {
	adminmiddleware.RegisterPermissionExempt("crud/log", "index")
	g.GET(logRoute+"/index", r.handler.Index)
}

func (r *LogRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
