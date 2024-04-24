//go:build wireinject
// +build wireinject

package main

import (
	"go-build-admin/conf"

	"go-build-admin/app/cron"
	"go-build-admin/router"
	"go-build-admin/service/db"
	"go-build-admin/service/rds"

	"go-build-admin/app/pkg/token"

	"github.com/google/wire"
	"go.uber.org/zap"
	"gopkg.in/natefinch/lumberjack.v2"
)

// wireApp init application.
func wireApp(*conf.Configuration, *lumberjack.Logger, *zap.Logger) (*App, func(), error) {
	panic(wire.Build(
		db.NewDB,
		rds.NewRedis,
		token.NewTokenHelper,
		ProviderSet,
		// middleware.ProviderSet,
		router.InitRouter,
		cron.ProviderSet,
		newHttpServer,
		newApp,
	))
}

// wireCommand init application.
// func wireCommand(*conf.Configuration, *lumberjack.Logger, *zap.Logger) (*cmd.Command, func(), error) {
// 	panic(wire.Build(
// 		cHandler.ProviderSet,
// 		cmd.NewCommand,
// 	))
// }
