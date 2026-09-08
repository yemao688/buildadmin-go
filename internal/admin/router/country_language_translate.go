// 业务定制 registrar：country.Language 模块的通用翻译工具接口
// （getMultTranslations）。主文件 country_language.go 由 CRUD 生成器模式
// 维护，不得写入自定义路由（形状守卫 TestRegistrarTemplateMatches
// CountryLanguageShape 会因偏离模板而变红）；自定义额外接口统一放独立
// registrar 文件，生成器重新生成主文件时本文件不受影响。
// 注意：若未来执行 `crud:delete country_language`，需同步删除本文件并移除
// provider.go 中 NewCountryLanguageTranslateRegistrar 的两处接线（否则残留
// 引用会在编译期暴露）。
package router

import (
	handler "buildadmin-go/internal/admin/handler"
	adminmiddleware "buildadmin-go/internal/admin/middleware"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

type CountryLanguageTranslateRegistrar struct {
	handler *handler.CountryLanguageHandler
}

func NewCountryLanguageTranslateRegistrar(handler *handler.CountryLanguageHandler) *CountryLanguageTranslateRegistrar {
	return &CountryLanguageTranslateRegistrar{handler: handler}
}

func (r *CountryLanguageTranslateRegistrar) Group() string { return "admin" }

func (r *CountryLanguageTranslateRegistrar) Register(g gin.IRoutes) {
	// getMultTranslations 为通用翻译工具接口（不挂业务菜单），声明式豁免
	// admin_rule 校验；CRUD 的 Index/Add/Edit/Del 仍走主文件的规则。
	adminmiddleware.RegisterHandlerExemptions("country.language", r.handler)
	g.POST(countryLanguageRoute+"/getMultTranslations", r.handler.GetMultTranslations)
}

func (r *CountryLanguageTranslateRegistrar) Capabilities() []middleware.AtomicRoute {
	return nil
}