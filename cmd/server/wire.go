//go:build wireinject
// +build wireinject

//go:generate go run -mod=mod github.com/google/wire/cmd/wire

package main

import (
	authHandler "buildadmin-go/internal/admin/handler/auth"
	countryHandler "buildadmin-go/internal/admin/handler/country"
	crudHandler "buildadmin-go/internal/admin/handler/crud"
	routineHandler "buildadmin-go/internal/admin/handler/routine"
	securityHandler "buildadmin-go/internal/admin/handler/security"
	userHandler "buildadmin-go/internal/admin/handler/user"
	crudModel "buildadmin-go/internal/model"
	authRepo "buildadmin-go/internal/admin/repository/auth"
	countryRepo "buildadmin-go/internal/admin/repository/country"
	routineRepo "buildadmin-go/internal/admin/repository/routine"
	securityRepo "buildadmin-go/internal/admin/repository/security"
	userRepo "buildadmin-go/internal/admin/repository/user"
	"buildadmin-go/internal/conf"

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
	"buildadmin-go/internal/cron"
	"buildadmin-go/internal/infra/db"
	"buildadmin-go/internal/infra/rds"
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
