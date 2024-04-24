package main

import (
	adminHandler "go-build-admin/app/admin/handler"
	adminModel "go-build-admin/app/admin/model"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(

	adminModel.NewAdminModel,
	adminModel.NewAdminLogModel,
	adminModel.NewTestBuildModel,
	adminModel.NewAuthModel,

	adminHandler.NewAdminHandler,
	adminHandler.NewAdminLogHandler,
	adminHandler.NewTestBuildHandler,
	adminHandler.NewIndexHandler,
)
