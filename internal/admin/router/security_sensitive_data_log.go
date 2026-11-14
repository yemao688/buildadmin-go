// Package router 注册 security_sensitive_data_log（敏感数据日志）模块路由，由 RouteRegistrar 模式维护（手写模块）。
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"net/http"

	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type SensitiveDataLogRegistrar struct {
	handler *handler.SensitiveDataLogHandler
}

func NewSensitiveDataLogRegistrar(handler *handler.SensitiveDataLogHandler) *SensitiveDataLogRegistrar {
	return &SensitiveDataLogRegistrar{handler: handler}
}

const sensitiveDataLogRoute = "security.SensitiveDataLog"

func (r *SensitiveDataLogRegistrar) Register(g gin.IRoutes) {
	g.GET(sensitiveDataLogRoute+"/index", r.handler.Index)
	g.GET(sensitiveDataLogRoute+"/info", r.handler.Info)
	g.POST(sensitiveDataLogRoute+"/rollback", r.handler.Rollback)
	g.DELETE(sensitiveDataLogRoute+"/del", r.handler.Del)
}

func (r *SensitiveDataLogRegistrar) Capabilities() []middleware.AtomicRoute {
	return []middleware.AtomicRoute{
		{Route: "security/sensitivedatalog", Action: "rollback", Method: http.MethodPost},
		{Route: "security/sensitivedatalog", Action: "del", Method: http.MethodDelete},
	}
}
