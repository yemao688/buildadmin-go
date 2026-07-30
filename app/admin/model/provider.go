package model

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewCrudLogModel,
	NewTableModel,
	NewTestBuildModel,
	NewUserGroupModel,
	NewUserMoneyLogModel,
	NewUserRuleModel,
	NewUserScoreLogModel,
	NewUserModel,
)
