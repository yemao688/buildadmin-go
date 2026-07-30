package user

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewUserModel,
	NewGroupModel,
	NewRuleModel,
	NewMoneyLogModel,
	NewScoreLogModel,
)
