package handler

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAjaxHandler,
	NewDashboardHandler,
	NewDashboardRegistrar,
	NewIndexHandler,
	NewTestBuildHandler,
	NewTestBuildRegistrar,
	NewModuleHandler,
	NewModuleRegistrar,
)
