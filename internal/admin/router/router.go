// Package router 装配 admin 渠道（/admin/*）路由。它接收 admin 渠道中间件
// （Login/Authorization/Security/Record）与后台 handler，负责：
//
//   - PermissionExempt 豁免登记（index index/logout、ajax *、alioss callback）
//   - 未受保护的后台入口（登录页、ajax 终端等，只经过全局中间件）
//   - 受保护的后台分组（Login → Authorization → Security 链）
//   - 模块 registrar 的 AtomicRoute 能力注册与路由挂载
//
// 模块路由注册器（registrar）按模块一个文件（internal/admin/router/<table>.go）
// 声明，聚合列表在 ProvideRegistrars——这是 CRUD 生成器的追加锚点：
// 新模块只在该函数里增加一个参数与返回 slice 中的一行。
//
// 启动权限诊断（Authorization.ReportUnprotectedRoutes）由组合根
// cmd/server/main.go 在 debug 环境驱动，依赖本包持有的 Authorization 中间件。
//
// Record 中间件属于 admin 渠道实现，但按既有语义挂载在引擎全局链上；
// 通过 RecordHandler 暴露给根装配件在全局 Use 序列中原位挂载。
package router

import (
	adminhandler "buildadmin-go/internal/admin/handler"
	adminMiddleware "buildadmin-go/internal/admin/middleware"
	"buildadmin-go/internal/middleware"

	"github.com/gin-gonic/gin"
)

// AdminRouterDeps 是 admin 渠道注册器的构造参数。Registrars 由
// ProvideRegistrars 聚合（wire 注入），不再经过根装配件分派。
type AdminRouterDeps struct {
	LoginM         *adminMiddleware.Login
	AuthorizationM *adminMiddleware.Authorization
	SecurityM      *adminMiddleware.Security
	RecordM        *adminMiddleware.Record
	IndexHandler   *adminhandler.IndexHandler
	AjaxHandler    *adminhandler.AjaxHandler
	Registrars     []Registrar
}

// AdminRouter 完成 /admin/* 路由与能力的注册。
type AdminRouter struct {
	deps AdminRouterDeps
}

func NewAdminRouter(deps AdminRouterDeps) *AdminRouter {
	return &AdminRouter{deps: deps}
}

// RecordHandler 返回 admin 渠道的 Record 中间件处理器。它不属于 /admin
// 分组，而是由根装配件挂在引擎全局 Use 序列上（语义与拆分前一致）。
func (r *AdminRouter) RecordHandler() gin.HandlerFunc {
	return r.deps.RecordM.Handler()
}

// Register 挂载全部后台路由与原子写能力。模块注册器由 deps.Registrars
// 提供（ProvideRegistrars 聚合，wire 注入）。
func (r *AdminRouter) Register(engine *gin.Engine) {
	// 声明式豁免：从 handler 的 NoNeedLoginer/NoNeedPermissioner 接口收集
	// （语义对齐 PHP 控制器的 $noNeedLogin/$noNeedPermission 属性）。
	// alioss/callback 挂在 AjaxHandler 上但豁免面不同，显式登记。
	adminMiddleware.RegisterHandlerExemptions("index", r.deps.IndexHandler)
	adminMiddleware.RegisterHandlerExemptions("ajax", r.deps.AjaxHandler)
	adminMiddleware.RegisterPermissionExempt("alioss", "callback")

	// 后台全部路由进入保护组（Login → Authorization → Security 链）；
	// 免登录 action 由中间件按 noNeedLogin 注册表逐 action 放行，不再
	// 有组外路由。
	adminRouter := engine.Group("/admin/").Use(
		r.deps.LoginM.Handler(),
		r.deps.AuthorizationM.Handler(),
		r.deps.SecurityM.Handler(),
	)
	adminRouter.GET("Index/index", r.deps.IndexHandler.Index)
	adminRouter.GET("Index/login", r.deps.IndexHandler.Login)
	adminRouter.POST("Index/login", r.deps.IndexHandler.Login)
	adminRouter.POST("Index/logout", r.deps.IndexHandler.Logout)
	adminRouter.GET("ajax/buildSuffixSvg", r.deps.AjaxHandler.BuildSuffixSvg)
	adminRouter.GET("ajax/terminal", r.deps.AjaxHandler.Terminal)

	adminRouter.GET("ajax/area", r.deps.AjaxHandler.Area)
	adminRouter.POST("ajax/upload", r.deps.AjaxHandler.Upload)
	adminRouter.POST("Alioss/callback", r.deps.AjaxHandler.AliossCallback)
	adminRouter.GET("ajax/getTablePk", r.deps.AjaxHandler.GetTablePk)
	adminRouter.GET("ajax/getTableList", r.deps.AjaxHandler.GetTableList)
	adminRouter.GET("ajax/getTableFieldList", r.deps.AjaxHandler.GetTableFieldList)
	adminRouter.GET("ajax/getDatabaseConnectionList", r.deps.AjaxHandler.GetDatabaseConnectionList)
	adminRouter.POST("ajax/clearCache", r.deps.AjaxHandler.ClearCache)
	adminRouter.POST("ajax/changeTerminalConfig", r.deps.AjaxHandler.ChangeTerminalConfig)

	// AtomicRoute 能力注册先于路由挂载，与拆分前的组装顺序保持一致。
	for _, registrar := range r.deps.Registrars {
		for _, capability := range registrar.Capabilities() {
			middleware.RegisterAtomicRoute(capability)
		}
	}
	for _, registrar := range r.deps.Registrars {
		registrar.Register(adminRouter)
	}
}
