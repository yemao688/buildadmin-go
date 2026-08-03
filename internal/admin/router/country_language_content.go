// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type CountryLanguageContentRegistrar struct {
	handler *handler.CountryLanguageContentHandler
}

func NewCountryLanguageContentRegistrar(handler *handler.CountryLanguageContentHandler) *CountryLanguageContentRegistrar {
	return &CountryLanguageContentRegistrar{handler: handler}
}

const countryLanguageContentRoute = "country.LanguageContent"

func (r *CountryLanguageContentRegistrar) Group() string { return "admin" }

func (r *CountryLanguageContentRegistrar) Register(g gin.IRoutes) {
	handler.CRUDRoutes(g, countryLanguageContentRoute, r.handler)
}

func (r *CountryLanguageContentRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(countryLanguageContentRoute)
}
