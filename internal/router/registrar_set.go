package router

import (
	api "buildadmin-go/internal/api/handler"
)

// ProvideRegistrars 聚合 api 渠道模块 registrar（admin 渠道的聚合与
// 挂载已收进 internal/admin/router.ProvideRegistrars / AdminRouter）。
// CRUD 生成器生成的 api 渠道 registrar 只在这里追加参数与条目。
func ProvideRegistrars(
	apiCommon *api.CommonRegistrar,
	apiUser *api.UserRegistrar,
) []RouteRegistrar {
	return []RouteRegistrar{
		apiCommon,
		apiUser,
	}
}
