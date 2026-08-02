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
	crudModel "go-build-admin/internal/admin/model/crud"
	authRepo "go-build-admin/internal/admin/repository/auth"
	countryRepo "go-build-admin/internal/admin/repository/country"
	routineRepo "go-build-admin/internal/admin/repository/routine"
	securityRepo "go-build-admin/internal/admin/repository/security"
	userRepo "go-build-admin/internal/admin/repository/user"
	"go-build-admin/internal/conf"

	adminHandler "go-build-admin/internal/admin/handler"
	adminMiddleware "go-build-admin/internal/admin/middleware"
	adminRepo "go-build-admin/internal/admin/repository"
	adminRouter "go-build-admin/internal/admin/router"
	apiHandler "go-build-admin/internal/api/handler"
	apiMiddleware "go-build-admin/internal/api/middleware"
	apiRouter "go-build-admin/internal/api/router"
	"go-build-admin/internal/api/service/member"
	"go-build-admin/internal/cmd"
	commandHandler "go-build-admin/internal/cmd/handler"
	"go-build-admin/internal/common/area"
	"go-build-admin/internal/common/money"
	"go-build-admin/internal/common/country"
	siteconfig "go-build-admin/internal/common/siteconfig"
	"go-build-admin/internal/common/upload"
	"go-build-admin/internal/cron"
	"go-build-admin/internal/infra/db"
	"go-build-admin/internal/infra/rds"
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
		apiMiddleware.ProviderSet,
		adminMiddleware.ProviderSet,
		area.ProviderSet,
		money.ProviderSet,
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
		adminRepo.ProviderSet,
		countryRepo.ProviderSet,
		authRepo.ProviderSet,
		routineRepo.ProviderSet,
		securityRepo.ProviderSet,
		userRepo.ProviderSet,
		crudModel.ProviderSet,
		wire.Bind(new(terminal.AuthModel), new(*authRepo.AuthRepository)),
		apiHandler.ProviderSet,

		router.ProvideRegistrars,
		adminRouter.NewAdminRouter,
		wire.Struct(new(adminRouter.AdminRouterDeps), "*"),
		apiRouter.NewApiRouter,
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
