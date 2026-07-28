package router

import admin "go-build-admin/app/admin/handler"

// ProvideRegistrars 聚合全部模块 registrar。CRUD 生成器只在这里追加参数与条目。
func ProvideRegistrars(
	countryLanguage *admin.CountryLanguageRegistrar,
	countryCurrency *admin.CountryCurrencyRegistrar,
	countryLanguageContent *admin.CountryLanguageContentRegistrar,
) []RouteRegistrar {
	return []RouteRegistrar{
		countryLanguage,
		countryCurrency,
		countryLanguageContent,
	}
}
