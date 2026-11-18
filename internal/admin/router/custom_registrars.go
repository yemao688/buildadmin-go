package router

import handler "buildadmin-go/internal/admin/handler"

// CustomRegistrarSet 与 ProvideRegistrars 的 []Registrar 区分类型，wire 才能
// 同时提供两者而不产生重复绑定。
type CustomRegistrarSet []Registrar

// ProvideCustomRegistrars 聚合手写 registrar（自定义 action、工具接口等）。
//
// 生成器只维护 ProvideRegistrars；自定义路由放这里，重新生成模块时不受影响。
// 新增自定义接口时：在 handler 包新增独立 handler 类型，在这里构造 registrar。
func ProvideCustomRegistrars(countryLanguageTranslate *handler.CountryLanguageTranslateHandler) CustomRegistrarSet {
	return CustomRegistrarSet{
		NewCountryLanguageTranslateRegistrar(countryLanguageTranslate),
	}
}
