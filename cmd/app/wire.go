//go:build wireinject
// +build wireinject

//go:generate go run -mod=mod github.com/google/wire/cmd/wire

package main

import (
	authHandler "go-build-admin/internal/admin/handler/auth"
	countryHandler "go-build-admin/internal/admin/handler/country"
	crudHandler "go-build-admin/internal/admin/handler/crud"
	routineHandler "go-build-admin/internal/admin/handler/routine"
	securityHandler "go-build-admin/internal/admin/handler/security"
	userHandler "go-build-admin/internal/admin/handler/user"
	authModel "go-build-admin/internal/admin/model/auth"
	countryModel "go-build-admin/internal/admin/model/country"
	crudModel "go-build-admin/internal/admin/model/crud"
	routineModel "go-build-admin/internal/admin/model/routine"
	securityModel "go-build-admin/internal/admin/model/security"
	userModel "go-build-admin/internal/admin/model/user"
	"go-build-admin/internal/conf"

	adminHandler "go-build-admin/internal/admin/handler"
	adminModel "go-build-admin/internal/admin/model"
	apiHandler "go-build-admin/internal/api/handler"
	"go-build-admin/internal/cmd"
	commandHandler "go-build-admin/internal/cmd/handler"
	"go-build-admin/internal/common/area"
	"go-build-admin/internal/common/country"
	"go-build-admin/internal/common/member"
	siteconfig "go-build-admin/internal/common/siteconfig"
	"go-build-admin/internal/common/upload"
	"go-build-admin/internal/cron"
	"go-build-admin/internal/infra/db"
	"go-build-admin/internal/infra/rds"
	"go-build-admin/internal/middleware"
	"go-build-admin/internal/pkg/terminal"
	"go-build-admin/internal/router"

	"go-build-admin/internal/pkg"

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
		area.ProviderSet,
		country.ProviderSet,
		upload.ProviderSet,
		siteconfig.ProviderSet,
		member.ProviderSet,
		adminHandler.ProviderSet,
		countryHandler.ProviderSet,
		authHandler.ProviderSet,
		routineHandler.ProviderSet,
		securityHandler.ProviderSet,
		userHandler.ProviderSet,
		crudHandler.ProviderSet,
		adminModel.ProviderSet,
		countryModel.ProviderSet,
		authModel.ProviderSet,
		routineModel.ProviderSet,
		securityModel.ProviderSet,
		userModel.ProviderSet,
		crudModel.ProviderSet,
		wire.Bind(new(terminal.AuthModel), new(*authModel.AuthModel)),
		apiHandler.ProviderSet,

		router.ProvideRegistrars,
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
