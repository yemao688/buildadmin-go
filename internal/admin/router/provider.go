package router

import (
	handler "buildadmin-go/internal/admin/handler"

	"github.com/google/wire"
)

// ProviderSet 聚合 admin 渠道路由注册器（registrar）构造器与
// ProvideRegistrars 聚合函数。CRUD 生成器生成的 registrar 构造器
// 在本集合中追加。
var ProviderSet = wire.NewSet(
	NewAdminRouter,
	NewLogRegistrar,
	NewModuleRegistrar,
	NewAdminGroupRegistrar,
	NewAdminRuleRegistrar,
	NewConfigRegistrar,
	NewAttachmentRegistrar,
	NewAdminRegistrar,
	NewUserRegistrar,
	NewDataRecycleRegistrar,
	NewDataRecycleLogRegistrar,
	NewSensitiveDataRegistrar,
	NewSensitiveDataLogRegistrar,
	NewAdminInfoRegistrar,
	NewAdminLogRegistrar,
	NewCrudRegistrar,
	NewDashboardRegistrar,
	NewUserLogRegistrar,
	ProvideRegistrars,
	NewCountryCurrencyRegistrar,
	NewCountryLanguageRegistrar,
	NewCountryLanguageContentRegistrar,
	NewCountryLanguageTranslateRegistrar,
)

// ProvideRegistrars 聚合全部 admin 模块 registrar，返回给 AdminRouter
// 在 Register 中统一完成能力登记与路由挂载。
//
// 这是 CRUD 生成器的追加锚点：新模块在这里增加一个 handler 参数，并在
// 返回 slice 中增加一行（保持"每模块一行"字面成立）。
func ProvideRegistrars(
	log *handler.LogHandler,
	module *handler.ModuleHandler,
	adminGroup *handler.AdminGroupHandler,
	adminRule *handler.AdminRuleHandler,
	config *handler.ConfigHandler,
	attachment *handler.AttachmentHandler,
	admin *handler.AdminHandler,
	user *handler.UserHandler,
	dataRecycle *handler.DataRecycleHandler,
	dataRecycleLog *handler.DataRecycleLogHandler,
	sensitiveData *handler.SensitiveDataHandler,
	sensitiveDataLog *handler.SensitiveDataLogHandler,
	adminInfo *handler.AdminInfoHandler,
	adminLog *handler.AdminLogHandler,
	crud *handler.CrudHandler,
	dashboard *handler.DashboardHandler,
	moneyLog *handler.MoneyLogHandler,
	countryCurrency *handler.CountryCurrencyHandler,
	countryLanguage *handler.CountryLanguageHandler,
	countryLanguageContent *handler.CountryLanguageContentHandler,
) []Registrar {
	return []Registrar{
		NewLogRegistrar(log),
		NewModuleRegistrar(module),
		NewAdminGroupRegistrar(adminGroup),
		NewAdminRuleRegistrar(adminRule),
		NewConfigRegistrar(config),
		NewAttachmentRegistrar(attachment),
		NewAdminRegistrar(admin),
		NewUserRegistrar(user),
		NewDataRecycleRegistrar(dataRecycle),
		NewDataRecycleLogRegistrar(dataRecycleLog),
		NewSensitiveDataRegistrar(sensitiveData),
		NewSensitiveDataLogRegistrar(sensitiveDataLog),
		NewAdminInfoRegistrar(adminInfo),
		NewAdminLogRegistrar(adminLog),
		NewCrudRegistrar(crud),
		NewDashboardRegistrar(dashboard),
		NewUserLogRegistrar(user, moneyLog),
		NewCountryCurrencyRegistrar(countryCurrency),
		NewCountryLanguageRegistrar(countryLanguage),
		NewCountryLanguageContentRegistrar(countryLanguageContent),
		NewCountryLanguageTranslateRegistrar(countryLanguage),
	}
}
