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
	NewCrudHandler,
	NewDashboardHandler,
	NewDataRecycleLogHandler,
	NewDataRecycleHandler,
	NewIndexHandler,
	NewSensitiveDataLogHandler,
	NewSensitiveDataHandler,
	NewTestBuildHandler,
	NewUserGroupHandler,
	NewUserMoneyLogHandler,
	NewUserRuleHandler,
	NewUserScoreLogHandler,
	NewUserHandler,
)
