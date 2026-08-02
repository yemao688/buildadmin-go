package auth

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewAdminModel,
	NewAdminGroupModel,
	NewAdminRuleModel,
	NewAdminLogModel,
	NewAuthModel,
)
