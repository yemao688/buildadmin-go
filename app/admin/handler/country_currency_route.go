// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package handler

import (
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
)

type CountryCurrencyRegistrar struct {
	handler *CountryCurrencyHandler
}

func NewCountryCurrencyRegistrar(handler *CountryCurrencyHandler) *CountryCurrencyRegistrar {
	return &CountryCurrencyRegistrar{handler: handler}
}

const countryCurrencyRoute = "countryCurrency"

func (r *CountryCurrencyRegistrar) Group() string { return "admin" }

func (r *CountryCurrencyRegistrar) Register(g gin.IRoutes) {
	CRUDRoutes(g, countryCurrencyRoute, r.handler)
}

func (r *CountryCurrencyRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(countryCurrencyRoute)
}
