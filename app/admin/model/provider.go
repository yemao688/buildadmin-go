package model

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAdminGroupModel,
	NewAdminLogModel,
	NewAdminRuleModel,
	NewAdminModel,
	NewAuthModel,
	NewConfigModel,
	NewCrudLogModel,
	NewDataRecycleLogModel,
	NewDataRecycleModel,
	NewSensitiveDataLogModel,
	NewSensitiveDataModel,
	NewTableModel,
	NewTestBuildModel,
	NewUserGroupModel,
	NewUserMoneyLogModel,
	NewUserRuleModel,
	NewUserScoreLogModel,
	NewUserModel,
)
