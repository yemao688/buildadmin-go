// Package router 装配 api 渠道路由：/install 安装向导、/api/install/* 安装
// API 与 /api/* 会员接口。它接收 api 渠道中间件（UserLogin）与 api handler，
// 完成公共路由挂载；InstallGuard 等全局中间件由根装配件在调用本包之前挂载。
//
// 模块路由注册器（registrar）按模块一个文件（internal/api/router/<module>.go）
// 声明，聚合列表在 ProvideRegistrars——与 admin 渠道同构的追加锚点：
// 新模块只在该函数里增加一个参数与返回 slice 中的一行。
package router

import (
	api "buildadmin-go/internal/api/handler"
	apiMiddleware "buildadmin-go/internal/api/middleware"
	"buildadmin-go/internal/utils"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// ApiRouterDeps 是 api 渠道注册器的构造参数。Registrars 由
// ProvideRegistrars 聚合（wire 注入），不再经过根装配件分派。
type ApiRouterDeps struct {
	UserLoginM     *apiMiddleware.UserLogin
	InstallHandler *api.InstallHandler
	Registrars     []Registrar
}

// ApiRouter 完成 /install 与 /api/* 路由的注册。
type ApiRouter struct {
	deps ApiRouterDeps
}

func NewApiRouter(deps ApiRouterDeps) *ApiRouter {
	return &ApiRouter{deps: deps}
}

// Register 挂载安装向导、安装 API 与会员 API 分组。模块注册器由 deps.Registrars
// 提供（ProvideRegistrars 聚合，wire 注入）。
func (r *ApiRouter) Register(engine *gin.Engine) {
	rootDir := utils.RootPath()

	// 安装向导页面与安装 API（只经过全局中间件，不进入 UserLogin）。
	engine.StaticFile("/install", filepath.Join(rootDir, "public/install/index.html"))
	engine.Static("/install", filepath.Join(rootDir, "public/install"))
	engine.POST("/api/install/changePackageManager", r.deps.InstallHandler.ChangePackageManager)
	engine.GET("/api/install/envBaseCheck", r.deps.InstallHandler.EnvBaseCheck)
	engine.POST("/api/install/envNpmCheck", r.deps.InstallHandler.EnvNpmCheck)
	engine.GET("/api/install/terminal", r.deps.InstallHandler.Terminal)
	engine.GET("/api/install/baseConfig", r.deps.InstallHandler.BaseConfig)
	engine.POST("/api/install/baseConfig", r.deps.InstallHandler.BaseConfig)
	engine.POST("/api/install/testDatabase", r.deps.InstallHandler.TestDatabase)
	engine.POST("/api/install/commandExecComplete", r.deps.InstallHandler.CommandExecComplete)
	engine.POST("/api/install/manualInstall", r.deps.InstallHandler.ManualInstall)
	engine.POST("/api/install/mvDist", r.deps.InstallHandler.MvDist)

	// 会员 API 分组：public 端点经 apiRouteSet 直挂引擎根（无 UserLogin），
	// 其余端点进入 UserLogin 链。
	apiRouter := engine.Group("/api/").Use(r.deps.UserLoginM.Handler())
	for _, registrar := range r.deps.Registrars {
		registrar.Register(newAPIRouteSet(engine, apiRouter))
	}
}
