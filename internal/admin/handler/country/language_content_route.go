// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package country

import (
	adminhandler "go-build-admin/internal/admin/handler"
	"go-build-admin/internal/middleware"

	"github.com/gin-gonic/gin"
)

type LanguageContentRegistrar struct {
	handler *LanguageContentHandler
}

func NewLanguageContentRegistrar(handler *LanguageContentHandler) *LanguageContentRegistrar {
	return &LanguageContentRegistrar{handler: handler}
}

const languageContentRoute = "country.LanguageContent"

func (r *LanguageContentRegistrar) Group() string { return "admin" }

func (r *LanguageContentRegistrar) Register(g gin.IRoutes) {
	adminhandler.CRUDRoutes(g, languageContentRoute, r.handler)
}

func (r *LanguageContentRegistrar) Capabilities() []middleware.AtomicRoute {
	return adminhandler.CRUDCapabilities(languageContentRoute)
}
