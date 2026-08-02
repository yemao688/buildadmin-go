package router

import (
	admin "buildadmin-go/internal/admin/handler"
	auth "buildadmin-go/internal/admin/handler/auth"
	country "buildadmin-go/internal/admin/handler/country"
	crud "buildadmin-go/internal/admin/handler/crud"
	routine "buildadmin-go/internal/admin/handler/routine"
	security "buildadmin-go/internal/admin/handler/security"
	user "buildadmin-go/internal/admin/handler/user"
	api "buildadmin-go/internal/api/handler"
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
