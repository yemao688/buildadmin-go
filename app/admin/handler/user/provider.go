package user

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewUserHandlerWithAuth,
	NewUserRegistrar,
	NewUserGroupHandlerWithAuth,
	NewUserGroupRegistrar,
	NewUserRuleHandlerWithAuth,
	NewUserRuleRegistrar,
	NewUserMoneyLogHandler,
	NewUserScoreLogHandler,
	NewUserLogRegistrar,
)
