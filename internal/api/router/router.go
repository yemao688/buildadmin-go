// Package router 装配 api 渠道路由：/api/* 会员接口。它接收 api 渠道中间件
// （UserLogin）与 api handler，完成公共路由挂载；全局中间件由根装配件在
// 调用本包之前挂载。
//
// 模块路由注册器（registrar）按模块一个文件（internal/api/router/<module>.go）
// 声明，聚合列表在 ProvideRegistrars——与 admin 渠道同构的追加锚点：
// 新模块只在该函数里增加一个参数与返回 slice 中的一行。
package router

import (
	apiMiddleware "buildadmin-go/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// ApiRouterDeps 是 api 渠道注册器的构造参数。Registrars 由
// ProvideRegistrars 聚合（wire 注入），不再经过根装配件分派。
type ApiRouterDeps struct {
	UserLoginM *apiMiddleware.UserLogin
	Registrars []Registrar
}

// ApiRouter 完成 /api/* 路由的注册。
type ApiRouter struct {
	deps ApiRouterDeps
}

func NewApiRouter(deps ApiRouterDeps) *ApiRouter {
	return &ApiRouter{deps: deps}
}

// Register 挂载会员 API 分组。模块注册器由 deps.Registrars 提供
// （ProvideRegistrars 聚合，wire 注入）。
func (r *ApiRouter) Register(engine *gin.Engine) {
	// 会员 API 分组：public 端点经 apiRouteSet 直挂引擎根（无 UserLogin），
	// 其余端点进入 UserLogin 链。
	apiRouter := engine.Group("/api/").Use(r.deps.UserLoginM.Handler())
	for _, registrar := range r.deps.Registrars {
		registrar.Register(newAPIRouteSet(engine, apiRouter))
	}
}
