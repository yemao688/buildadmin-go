// 由 RouteRegistrar 模式维护（手写模块）
package security

import (
	adminhandler "go-build-admin/app/admin/handler"
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type DataRecycleRegistrar struct {
	handler *DataRecycleHandler
}

func NewDataRecycleRegistrar(handler *DataRecycleHandler) *DataRecycleRegistrar {
	return &DataRecycleRegistrar{handler: handler}
}

const dataRecycleRoute = "security.DataRecycle"

func (r *DataRecycleRegistrar) Group() string { return "admin" }

func (r *DataRecycleRegistrar) Register(g gin.IRoutes) {
	g.GET(dataRecycleRoute+"/index", r.handler.Index)
	g.GET(dataRecycleRoute+"/add", r.handler.Add)
	g.POST(dataRecycleRoute+"/add", r.handler.Add)
	g.GET(dataRecycleRoute+"/edit", r.handler.One)
	g.POST(dataRecycleRoute+"/edit", r.handler.Edit)
	g.DELETE(dataRecycleRoute+"/del", r.handler.Del)
}

func (r *DataRecycleRegistrar) Capabilities() []middleware.AtomicRoute {
	return adminhandler.CRUDCapabilities(dataRecycleRoute)
}
