// 由 RouteRegistrar 模式维护（手写模块）
package routine

import (
	adminhandler "go-build-admin/app/admin/handler"
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type ConfigRegistrar struct {
	handler *ConfigHandler
}

func NewConfigRegistrar(handler *ConfigHandler) *ConfigRegistrar {
	return &ConfigRegistrar{handler: handler}
}

const configRoute = "routine.Config"

func (r *ConfigRegistrar) Group() string { return "admin" }

func (r *ConfigRegistrar) Register(g gin.IRoutes) {
	g.GET(configRoute+"/index", r.handler.Index)
	g.POST(configRoute+"/add", r.handler.Add)
	g.POST(configRoute+"/edit", r.handler.Edit)
	g.DELETE(configRoute+"/del", r.handler.Del)
	g.POST(configRoute+"/sendTestMail", r.handler.SendTestMail)
}

func (r *ConfigRegistrar) Capabilities() []middleware.AtomicRoute {
	return adminhandler.CRUDCapabilities(configRoute)
}
