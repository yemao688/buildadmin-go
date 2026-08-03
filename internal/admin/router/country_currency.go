// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type CountryCurrencyRegistrar struct {
	handler *handler.CountryCurrencyHandler
}

func NewCountryCurrencyRegistrar(handler *handler.CountryCurrencyHandler) *CountryCurrencyRegistrar {
	return &CountryCurrencyRegistrar{handler: handler}
}

const countryCurrencyRoute = "country.Currency"

func (r *CountryCurrencyRegistrar) Group() string { return "admin" }

func (r *CountryCurrencyRegistrar) Register(g gin.IRoutes) {
	handler.CRUDRoutes(g, countryCurrencyRoute, r.handler)
}

func (r *CountryCurrencyRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(countryCurrencyRoute)
}
