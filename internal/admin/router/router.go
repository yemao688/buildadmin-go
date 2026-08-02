// Package router 装配 admin 渠道（/admin/*）路由。它接收 admin 渠道中间件
// （Login/Authorization/Security/Record）与后台 handler/registrars，负责：
//
//   - PermissionExempt 豁免登记（index index/logout、ajax *、alioss callback）
//   - 未受保护的后台入口（登录页、ajax 终端等，只经过全局中间件）
//   - 受保护的后台分组（Login → Authorization → Security 链）
//   - 模块 registrar 的 AtomicRoute 能力注册与路由挂载
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

// AdminRouterDeps 是 admin 渠道注册器的构造参数。Registrars 由根装配件
// 按 Group() 分派后通过 Register 传入，不参与 wire 注入。
type AdminRouterDeps struct {
	LoginM         *adminMiddleware.Login
	AuthorizationM *adminMiddleware.Authorization
	SecurityM      *adminMiddleware.Security
	RecordM        *adminMiddleware.Record
	IndexHandler   *adminhandler.IndexHandler
	AjaxHandler    *adminhandler.AjaxHandler
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

// Register 挂载全部后台路由与原子写能力。registrars 是 Group()=="admin"
// 的模块注册器，由根装配件分派后传入。
func (r *AdminRouter) Register(engine *gin.Engine, registrars []Registrar) {
	// 豁免清单：登录、ajax 与 alioss 回调不要求后台登录/权限。
	adminMiddleware.RegisterPermissionExempt("index", "index", "logout")
	adminMiddleware.RegisterPermissionExempt("ajax", "*")
	adminMiddleware.RegisterPermissionExempt("alioss", "callback")

	// 未受保护的后台入口（只经过全局中间件，不进入 Login/Authorization/Security）。
	engine.GET("/admin/Index/login", r.deps.IndexHandler.Login)
	engine.POST("/admin/Index/login", r.deps.IndexHandler.Login)
	engine.GET("/admin/ajax/buildSuffixSvg", r.deps.AjaxHandler.BuildSuffixSvg)
	engine.GET("/admin/ajax/terminal", r.deps.AjaxHandler.Terminal)

	// 受保护的后台分组。
	adminRouter := engine.Group("/admin/").Use(
		r.deps.LoginM.Handler(),
		r.deps.AuthorizationM.Handler(),
		r.deps.SecurityM.Handler(),
	)
	adminRouter.GET("Index/index", r.deps.IndexHandler.Index)
	adminRouter.POST("Index/logout", r.deps.IndexHandler.Logout)

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
	for _, registrar := range registrars {
		for _, capability := range registrar.Capabilities() {
			middleware.RegisterAtomicRoute(capability)
		}
	}
	for _, registrar := range registrars {
		registrar.Register(adminRouter)
	}
}
