package handler

import (
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	NewAccountHandler,
	NewAccountRegistrar,
	NewAjaxHandler,
	NewAjaxRegistrar,
	NewCommonHandler,
	NewCommonRegistrar,
	NewEmsHandler,
	NewEmsRegistrar,
	NewIndexHandler,
	NewIndexRegistrar,
	NewInstallHandler,
	NewUserHandler,
	NewUserRegistrar,
	NewDemoHandler,
	NewDemoRegistrar,
)
