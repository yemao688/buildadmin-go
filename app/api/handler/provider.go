package handler

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAccountHandler,
	NewAjaxHandler,
	NewCommonHandler,
	NewEmsHandler,
	NewIndexHandler,
	NewInstallHandler,
	NewUserHandler,
	NewDemoHandler,
)
