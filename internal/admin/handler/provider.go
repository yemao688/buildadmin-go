package handler

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAjaxHandler,
	NewDashboardHandler,
	NewDashboardRegistrar,
	NewIndexHandler,
	NewModuleHandler,
	NewModuleRegistrar,
)
