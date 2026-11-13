package router

import (
	handler "buildadmin-go/internal/admin/handler"

	"github.com/google/wire"
)

// ProviderSet 聚合 admin 渠道的 bootstrap 与 registrar 聚合函数。
// registrar 构造器不再进入本集合：生成器只维护 ProvideRegistrars 的
// 参数与返回条目，每表 registrar 在此函数内构造。
var ProviderSet = wire.NewSet(
	NewAdminRouter,
	ProvideRegistrars,
)

// ProvideRegistrars 聚合全部 admin 模块 registrar，返回给 AdminRouter
// 在 Register 中统一完成能力登记与路由挂载。
//
// 这是 CRUD 生成器的追加锚点：新模块在这里增加一个 handler 参数，并在
// 返回 slice 中增加一行（保持"每模块一行"字面成立）。
func ProvideRegistrars(
	log *handler.LogHandler,
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
