// 由 CRUD 生成器模式维护，自定义额外接口请新增独立 registrar 文件。
package country

import (
	adminhandler "buildadmin-go/internal/admin/handler"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type CurrencyRegistrar struct {
	handler *CurrencyHandler
}

func NewCurrencyRegistrar(handler *CurrencyHandler) *CurrencyRegistrar {
	return &CurrencyRegistrar{handler: handler}
}

const currencyRoute = "country.Currency"

func (r *CurrencyRegistrar) Group() string { return "admin" }

func (r *CurrencyRegistrar) Register(g gin.IRoutes) {
	adminhandler.CRUDRoutes(g, currencyRoute, r.handler)
}

func (r *CurrencyRegistrar) Capabilities() []middleware.AtomicRoute {
	return adminhandler.CRUDCapabilities(currencyRoute)
}
