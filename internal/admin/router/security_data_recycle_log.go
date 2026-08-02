// 由 RouteRegistrar 模式维护（手写模块）
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"net/http"

	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type DataRecycleLogRegistrar struct {
	handler *handler.DataRecycleLogHandler
}

func NewDataRecycleLogRegistrar(handler *handler.DataRecycleLogHandler) *DataRecycleLogRegistrar {
	return &DataRecycleLogRegistrar{handler: handler}
}

const dataRecycleLogRoute = "security.DataRecycleLog"

func (r *DataRecycleLogRegistrar) Group() string { return "admin" }

func (r *DataRecycleLogRegistrar) Register(g gin.IRoutes) {
	g.GET(dataRecycleLogRoute+"/index", r.handler.Index)
	g.GET(dataRecycleLogRoute+"/info", r.handler.Info)
	g.POST(dataRecycleLogRoute+"/restore", r.handler.Restore)
	g.DELETE(dataRecycleLogRoute+"/del", r.handler.Del)
}

func (r *DataRecycleLogRegistrar) Capabilities() []middleware.AtomicRoute {
	return []middleware.AtomicRoute{
		{Route: "security/datarecyclelog", Action: "restore", Method: http.MethodPost},
		{Route: "security/datarecyclelog", Action: "del", Method: http.MethodDelete},
	}
}
