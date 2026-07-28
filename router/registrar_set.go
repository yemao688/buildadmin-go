package router

import admin "go-build-admin/app/admin/handler"

// ProvideRegistrars 聚合全部模块 registrar。CRUD 生成器只在这里追加参数与条目。
func ProvideRegistrars(
	countryLanguage *admin.CountryLanguageRegistrar,
	countryCurrency *admin.CountryCurrencyRegistrar,
	countryLanguageContent *admin.CountryLanguageContentRegistrar,
	crudLog *admin.CrudLogRegistrar,
	module *admin.ModuleRegistrar,
	testBuild *admin.TestBuildRegistrar,
	adminGroup *admin.AdminGroupRegistrar,
	adminRule *admin.AdminRuleRegistrar,
	userGroup *admin.UserGroupRegistrar,
	userRule *admin.UserRuleRegistrar,
	config *admin.ConfigRegistrar,
	attachment *admin.AttachmentRegistrar,
	admin *admin.AdminRegistrar,
	user *admin.UserRegistrar,
	dataRecycle *admin.DataRecycleRegistrar,
	dataRecycleLog *admin.DataRecycleLogRegistrar,
	sensitiveData *admin.SensitiveDataRegistrar,
	sensitiveDataLog *admin.SensitiveDataLogRegistrar,
) []RouteRegistrar {
	return []RouteRegistrar{
		countryLanguage,
		countryCurrency,
		countryLanguageContent,
		crudLog,
		module,
		testBuild,
		adminGroup,
		adminRule,
		userGroup,
		userRule,
		config,
		attachment,
		admin,
		user,
		dataRecycle,
		dataRecycleLog,
		sensitiveData,
		sensitiveDataLog,
	}
}
