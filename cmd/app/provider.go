package main

import (
	adminHandler "go-build-admin/app/admin/handler"
	adminModel "go-build-admin/app/admin/model"
	apiHandler "go-build-admin/app/api/handler"
	"go-build-admin/app/middleware"
	"go-build-admin/app/pkg/clickcaptcha"

	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	clickcaptcha.NewCaptcha,
	middleware.NewAuth,
	middleware.NewPermission,

	adminHandler.NewAdminHandler,
	adminHandler.NewDashboardHandler,
	adminHandler.NewAdminLogHandler,
	adminHandler.NewTestBuildHandler,
	adminHandler.NewIndexHandler,

	adminModel.NewAdminModel,
	adminModel.NewAdminRuleModel,
	adminModel.NewAdminLogModel,
	adminModel.NewTestBuildModel,
	adminModel.NewAuthModel,

	apiHandler.NewAccountHandler,
	apiHandler.NewAjaxHandler,
	apiHandler.NewCommonHandler,
	apiHandler.NewEmsHandler,
	apiHandler.NewIndexHandler,
	apiHandler.NewInstallHandler,
	apiHandler.NewUserHandler,
)
