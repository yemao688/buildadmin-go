package router

import (
	admin "go-build-admin/app/admin/handler"
	auth "go-build-admin/app/admin/handler/auth"
	country "go-build-admin/app/admin/handler/country"
	crud "go-build-admin/app/admin/handler/crud"
	routine "go-build-admin/app/admin/handler/routine"
	security "go-build-admin/app/admin/handler/security"
	user "go-build-admin/app/admin/handler/user"
	api "go-build-admin/app/api/handler"
)

// ProvideRegistrars 聚合全部模块 registrar。CRUD 生成器只在这里追加参数与条目。
func ProvideRegistrars(
	crudLog *crud.CrudLogRegistrar,
	module *admin.ModuleRegistrar,
	testBuild *admin.TestBuildRegistrar,
	adminGroup *auth.AdminGroupRegistrar,
	adminRule *auth.AdminRuleRegistrar,
	userGroup *user.UserGroupRegistrar,
	userRule *user.UserRuleRegistrar,
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
	apiAccount *api.AccountRegistrar,
	apiAjax *api.AjaxRegistrar,
	apiCommon *api.CommonRegistrar,
	apiEms *api.EmsRegistrar,
	apiIndex *api.IndexRegistrar,
	apiUser *api.UserRegistrar,
	apiDemo *api.DemoRegistrar,
	countryCurrencyRegistrar *country.CurrencyRegistrar,
	countryLanguageRegistrar *country.LanguageRegistrar,
	countryLanguageContentRegistrar *country.LanguageContentRegistrar,
) []RouteRegistrar {
	return []RouteRegistrar{
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
		adminInfo,
		adminLog,
		crud,
		dashboard,
		userLog,
		apiAccount,
		apiAjax,
		apiCommon,
		apiEms,
		apiIndex,
		apiUser,
		apiDemo,
		countryCurrencyRegistrar,
		countryLanguageRegistrar,
		countryLanguageContentRegistrar,
	}
}
