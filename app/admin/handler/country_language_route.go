// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type CountryLanguageRegistrar struct {
	handler *CountryLanguageHandler
}

func NewCountryLanguageRegistrar(handler *CountryLanguageHandler) *CountryLanguageRegistrar {
	return &CountryLanguageRegistrar{handler: handler}
}

const countryLanguageRoute = "countryLanguage"

func (r *CountryLanguageRegistrar) Group() string { return "admin" }

func (r *CountryLanguageRegistrar) Register(g gin.IRoutes) {
	CRUDRoutes(g, countryLanguageRoute, r.handler)
}

func (r *CountryLanguageRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(countryLanguageRoute)
}
