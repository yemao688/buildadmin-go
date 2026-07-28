package handler

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAdminGroupHandler,
	NewAdminGroupRegistrar,
	NewAdminInfoHandler,
	NewAdminLogHandler,
	NewAdminRuleHandler,
	NewAdminRuleRegistrar,
	NewAdminHandler,
	NewAjaxHandler,
	NewAttachmentHandler,
	NewAttachmentRegistrar,
	NewConfigHandler,
	NewConfigRegistrar,
	NewCrudLogHandler,
	NewCrudLogRegistrar,
	NewCrudHandler,
	NewDashboardHandler,
	NewDataRecycleLogHandler,
	NewDataRecycleHandler,
	NewIndexHandler,
	NewSensitiveDataLogHandler,
	NewSensitiveDataHandler,
	NewTestBuildHandler,
	NewTestBuildRegistrar,
	NewUserGroupHandler,
	NewUserGroupRegistrar,
	NewUserMoneyLogHandler,
	NewUserRuleHandler,
	NewUserRuleRegistrar,
	NewUserScoreLogHandler,
	NewUserHandler,
	NewModuleHandler,
	NewModuleRegistrar,

	NewCountryLanguageHandler,
	NewCountryLanguageRegistrar,

	NewCountryCurrencyHandler,
	NewCountryCurrencyRegistrar,
	NewCountryLanguageContentHandler,
	NewCountryLanguageContentRegistrar,
)
