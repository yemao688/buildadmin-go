// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type LanguageRegistrar struct {
	handler *handler.LanguageHandler
}

func NewLanguageRegistrar(handler *handler.LanguageHandler) *LanguageRegistrar {
	return &LanguageRegistrar{handler: handler}
}

const languageRoute = "country.Language"

func (r *LanguageRegistrar) Group() string { return "admin" }

func (r *LanguageRegistrar) Register(g gin.IRoutes) {
	handler.CRUDRoutes(g, languageRoute, r.handler)
}

func (r *LanguageRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(languageRoute)
}
