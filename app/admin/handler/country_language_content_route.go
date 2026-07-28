// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type CountryLanguageContentRegistrar struct {
	handler *CountryLanguageContentHandler
}

func NewCountryLanguageContentRegistrar(handler *CountryLanguageContentHandler) *CountryLanguageContentRegistrar {
	return &CountryLanguageContentRegistrar{handler: handler}
}

const countryLanguageContentRoute = "countryLanguageContent"

func (r *CountryLanguageContentRegistrar) Group() string { return "admin" }

func (r *CountryLanguageContentRegistrar) Register(g gin.IRoutes) {
	CRUDRoutes(g, countryLanguageContentRoute, r.handler)
}

func (r *CountryLanguageContentRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(countryLanguageContentRoute)
}
