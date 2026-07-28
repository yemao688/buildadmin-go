package handler

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAdminGroupHandler,
	NewAdminInfoHandler,
	NewAdminLogHandler,
	NewAdminRuleHandler,
	NewAdminHandler,
	NewAjaxHandler,
	NewAttachmentHandler,
	NewConfigHandler,
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
	NewUserMoneyLogHandler,
	NewUserRuleHandler,
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
