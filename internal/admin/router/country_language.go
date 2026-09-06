// 由 CRUD 生成器模式维护（额外接口 getMultTranslations 为业务定制，重新
// 生成后需按 git diff 回补；框架约定见 AGENTS.md 双提交工作流）。
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	adminmiddleware "buildadmin-go/internal/admin/middleware"
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
	// getMultTranslations 为通用翻译工具接口（不挂业务菜单），声明式豁免
	// admin_rule 校验；Index/Add/Edit/Del 仍走 CRUD 规则。
	adminmiddleware.RegisterHandlerExemptions("country.language", r.handler)
	CRUDRoutes(g, countryLanguageRoute, r.handler)
	g.POST(countryLanguageRoute+"/getMultTranslations", r.handler.GetMultTranslations)
}

func (r *CountryLanguageRegistrar) Capabilities() []middleware.AtomicRoute {
	return CRUDCapabilities(countryLanguageRoute)
}