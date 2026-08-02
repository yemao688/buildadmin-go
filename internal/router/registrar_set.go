package router

import (
	admin "go-build-admin/internal/admin/handler"
	auth "go-build-admin/internal/admin/handler/auth"
	country "go-build-admin/internal/admin/handler/country"
	crud "go-build-admin/internal/admin/handler/crud"
	routine "go-build-admin/internal/admin/handler/routine"
	security "go-build-admin/internal/admin/handler/security"
	user "go-build-admin/internal/admin/handler/user"
	api "go-build-admin/internal/api/handler"
)

// ProvideRegistrars 聚合全部模块 registrar。CRUD 生成器只在这里追加参数与条目。
func ProvideRegistrars(
	log *crud.LogRegistrar,
	module *admin.ModuleRegistrar,
	adminGroup *auth.AdminGroupRegistrar,
	adminRule *auth.AdminRuleRegistrar,
	config *routine.ConfigRegistrar,
	attachment *routine.AttachmentRegistrar,
	admin *auth.AdminRegistrar,
	user *user.UserRegistrar,
	dataRecycle *security.DataRecycleRegistrar,
	dataRecycleLog *security.DataRecycleLogRegistrar,
	sensitiveData *security.SensitiveDataRegistrar,
	sensitiveDataLog *security.SensitiveDataLogRegistrar,
	adminInfo *routine.AdminInfoRegistrar,
	adminLog *auth.AdminLogRegistrar,
	crud *crud.CrudRegistrar,
	dashboard *admin.DashboardRegistrar,
	userLog *user.UserLogRegistrar,
	apiCommon *api.CommonRegistrar,
	apiUser *api.UserRegistrar,
	countryCurrencyRegistrar *country.CurrencyRegistrar,
	countryLanguageRegistrar *country.LanguageRegistrar,
	countryLanguageContentRegistrar *country.LanguageContentRegistrar,
) []RouteRegistrar {
	return []RouteRegistrar{
		log,
		module,
		adminGroup,
		adminRule,
		config,
		attachment,
		admin,
		user,
		dataRecycle,
		dataRecycleLog,
		sensitiveData,
		sensitiveDataLog,
		adminInfo,
		adminLog,
		crud,
		dashboard,
		userLog,
		apiCommon,
		apiUser,
		countryCurrencyRegistrar,
		countryLanguageRegistrar,
		countryLanguageContentRegistrar,
	}
}
