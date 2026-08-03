// Package install 装配安装渠道：/install 安装向导页面与 /api/install/* 安装
// API。与 admin/api 渠道同构的自注册形态：InstallRouter 持有 handler 依赖，
// Register 在引擎上统一挂载；InstallGuard 等全局中间件由根装配件在调用
// 本包之前挂载。
package install

import (
	"buildadmin-go/internal/utils"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// InstallRouterDeps 是安装渠道注册器的构造参数。
type InstallRouterDeps struct {
	InstallHandler *InstallHandler
}

// InstallRouter 完成 /install 与 /api/install/* 路由的注册。
type InstallRouter struct {
	deps InstallRouterDeps
}

func NewInstallRouter(deps InstallRouterDeps) *InstallRouter {
	return &InstallRouter{deps: deps}
}

// Register 挂载安装向导页面与安装 API（只经过全局中间件，不进入 UserLogin）。
func (r *InstallRouter) Register(engine *gin.Engine) {
	rootDir := utils.RootPath()

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
}
