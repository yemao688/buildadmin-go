package auth

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAdminHandler,
	NewAdminRegistrar,
	NewAdminGroupHandler,
	NewAdminGroupRegistrar,
	NewAdminRuleHandler,
	NewAdminRuleRegistrar,
	NewAdminLogHandler,
	NewAdminLogRegistrar,
)
