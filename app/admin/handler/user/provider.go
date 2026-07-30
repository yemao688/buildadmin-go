package user

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewUserHandlerWithAuth,
	NewUserRegistrar,
	NewGroupHandlerWithAuth,
	NewGroupRegistrar,
	NewRuleHandlerWithAuth,
	NewRuleRegistrar,
	NewMoneyLogHandler,
	NewScoreLogHandler,
	NewUserLogRegistrar,
)
