package auth

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAdminRepository,
	NewAdminGroupRepository,
	NewAdminRuleRepository,
	NewAdminLogRepository,
	NewAuthRepository,
)
