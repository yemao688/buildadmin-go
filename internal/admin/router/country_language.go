// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type CountryLanguageRegistrar struct {
	handler *handler.CountryLanguageHandler
}

func NewCountryLanguageRegistrar(handler *handler.CountryLanguageHandler) *CountryLanguageRegistrar {
	return &CountryLanguageRegistrar{handler: handler}
}

const countryLanguageRoute = "country.Language"

func (r *CountryLanguageRegistrar) Group() string { return "admin" }

func (r *CountryLanguageRegistrar) Register(g gin.IRoutes) {
	handler.CRUDRoutes(g, countryLanguageRoute, r.handler)
}

func (r *CountryLanguageRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(countryLanguageRoute)
}
