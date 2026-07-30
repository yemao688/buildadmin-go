package handler

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAjaxHandler,
	NewCrudLogHandler,
	NewCrudLogRegistrar,
	NewCrudHandler,
	NewCrudRegistrar,
	NewDashboardHandler,
	NewDashboardRegistrar,
	NewIndexHandler,
	NewTestBuildHandler,
	NewTestBuildRegistrar,
	NewUserGroupHandlerWithAuth,
	NewUserGroupRegistrar,
	NewUserMoneyLogHandler,
	NewUserRuleHandlerWithAuth,
	NewUserRuleRegistrar,
	NewUserScoreLogHandler,
	NewUserLogRegistrar,
	NewUserHandlerWithAuth,
	NewUserRegistrar,
	NewModuleHandler,
	NewModuleRegistrar,
)
