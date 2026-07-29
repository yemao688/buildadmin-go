// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package country

import (
	adminhandler "go-build-admin/app/admin/handler"
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type LanguageRegistrar struct {
	handler *LanguageHandler
}

func NewLanguageRegistrar(handler *LanguageHandler) *LanguageRegistrar {
	return &LanguageRegistrar{handler: handler}
}

const languageRoute = "country.Language"

func (r *LanguageRegistrar) Group() string { return "admin" }

func (r *LanguageRegistrar) Register(g gin.IRoutes) {
	adminhandler.CRUDRoutes(g, languageRoute, r.handler)
}

func (r *LanguageRegistrar) Capabilities() []middleware.AtomicRoute {
	return adminhandler.CRUDCapabilities(languageRoute)
}
