// 由 RouteRegistrar 模式维护（手写模块）
package crud

import (
	adminmiddleware "buildadmin-go/internal/admin/middleware"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type LogRegistrar struct {
	handler *LogHandler
}

func NewLogRegistrar(handler *LogHandler) *LogRegistrar {
	return &LogRegistrar{handler: handler}
}

const logRoute = "crud.Log"

func (r *LogRegistrar) Group() string { return "admin" }

func (r *LogRegistrar) Register(g gin.IRoutes) {
	adminmiddleware.RegisterPermissionExempt("crud/log", "index")
	g.GET(logRoute+"/index", r.handler.Index)
}

func (r *LogRegistrar) Capabilities() []middleware.AtomicRoute { return nil }
