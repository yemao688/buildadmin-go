//go:build wireinject
// +build wireinject

//go:generate go run -mod=mod github.com/google/wire/cmd/wire

package main

import (
	adminHandler "buildadmin-go/internal/admin/handler"
	adminMiddleware "buildadmin-go/internal/admin/middleware"
	adminRepo "buildadmin-go/internal/admin/repository"
	adminRouter "buildadmin-go/internal/admin/router"
	apiHandler "buildadmin-go/internal/api/handler"
	apiMiddleware "buildadmin-go/internal/api/middleware"
	apiRouter "buildadmin-go/internal/api/router"
	"buildadmin-go/internal/api/service/member"
	"buildadmin-go/internal/cmd"
	commandHandler "buildadmin-go/internal/cmd/handler"
	"buildadmin-go/internal/common/area"
	"buildadmin-go/internal/common/country"
	"buildadmin-go/internal/common/money"
	siteconfig "buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/common/upload"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/cron"
	"buildadmin-go/internal/infra/db"
	"buildadmin-go/internal/infra/rds"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/terminal"
	"buildadmin-go/internal/router"

	"buildadmin-go/internal/pkg"

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
		apiMiddleware.ProviderSet,
		adminMiddleware.ProviderSet,
		area.ProviderSet,
		money.ProviderSet,
		country.ProviderSet,
		upload.ProviderSet,
		siteconfig.ProviderSet,
		member.ProviderSet,
		adminHandler.ProviderSet,
		adminRepo.ProviderSet,
		model.ProviderSet,
		wire.Bind(new(terminal.AuthModel), new(*adminRepo.AuthRepository)),
		apiHandler.ProviderSet,

		adminRouter.ProviderSet,
		wire.Struct(new(adminRouter.AdminRouterDeps), "*"),
		apiRouter.ProviderSet,
		wire.Struct(new(apiRouter.ApiRouterDeps), "*"),
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
