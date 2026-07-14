//go:build wireinject
// +build wireinject

//go:generate go run -mod=mod github.com/google/wire/cmd/wire

package main

import (
	"go-build-admin/conf"

	adminHandler "go-build-admin/app/admin/handler"
	adminModel "go-build-admin/app/admin/model"
	apiHandler "go-build-admin/app/api/handler"
	"go-build-admin/app/cmd"
	commandHandler "go-build-admin/app/cmd/handler"
	commonModel "go-build-admin/app/common/model"
	"go-build-admin/app/cron"
	"go-build-admin/app/middleware"
	"go-build-admin/router"
	"go-build-admin/service/db"
	"go-build-admin/service/rds"

	"go-build-admin/app/pkg"

	"github.com/google/wire"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

// wireApp init application.
func wireApp(*conf.Configuration, *lumberjack.Logger, *zap.Logger) (*App, func(), error) {
	panic(wire.Build(
		db.NewDB,
		rds.NewRedis,

		pkg.ProviderSet,
		middleware.ProviderSet,
		commonModel.ProviderSet,
		adminHandler.ProviderSet,
		adminModel.ProviderSet,
		apiHandler.ProviderSet,

		router.InitRouter,
		cron.ProviderSet,
		newHttpServer,
		newApp,
	))
}

// wireCommand init application.
func wireCommand(*conf.Configuration, *lumberjack.Logger, *zap.Logger) (*cmd.Command, func(), error) {
	panic(wire.Build(
		commandHandler.ProviderSet,
		cmd.NewCommand,
	))
}
