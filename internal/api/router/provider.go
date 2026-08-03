package router

import (
	api "buildadmin-go/internal/api/handler"

	"github.com/google/wire"
)

// ProviderSet 聚合 api 渠道路由注册器（registrar）构造器与
// ProvideRegistrars 聚合函数。手写模块的 registrar 构造器在本集合中追加。
var ProviderSet = wire.NewSet(
	NewApiRouter,
	NewCommonRegistrar,
	NewUserRegistrar,
	ProvideRegistrars,
)

// ProvideRegistrars 聚合全部 api 模块 registrar，返回给 ApiRouter
// 在 Register 中统一完成路由挂载。
func ProvideRegistrars(
	apiCommon *api.CommonHandler,
	apiUser *api.UserHandler,
) []Registrar {
	return []Registrar{
		NewCommonRegistrar(apiCommon),
		NewUserRegistrar(apiUser),
	}
}
