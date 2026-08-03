package router

import (
	admin "buildadmin-go/internal/admin/handler"
	adminRouter "buildadmin-go/internal/admin/router"
	apiRouter "buildadmin-go/internal/api/router"
	"buildadmin-go/internal/i18n"
	installRouter "buildadmin-go/internal/install"
	"buildadmin-go/internal/middleware"
	"buildadmin-go/internal/utils"
	"net/http"
	"os"
	"path/filepath"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitRouter 是根装配件：创建 gin.Engine、挂载全局中间件（Cors/Record/
// Logger/CustomRecovery/i18n/InstallGuard）与静态资源，然后调用 admin、api
// 与 install 三个渠道注册器完成挂载。各渠道的模块 registrar 已由渠道包
// 自行聚合注入（internal/admin/router.ProvideRegistrars、
// internal/api/router.ProvideRegistrars），根装配件不再按 Group() 分派。
func InitRouter(
	loggerWriter *lumberjack.Logger,
	adminR *adminRouter.AdminRouter,
	apiR *apiRouter.ApiRouter,
	installR *installRouter.InstallRouter,
) *gin.Engine {
	router := gin.New()
	registerHealthRoute(router)

	// 跨域处理。Record 是 admin 渠道的中间件，但按既有语义挂在全局链上。
	router.Use(middleware.Cors(), adminR.RecordHandler())
	router.Use(
		gin.Logger(),
		middleware.CustomRecovery(loggerWriter),
		//开启多语言
		ginI18n.Localize(ginI18n.WithBundle(i18n.NewBundleCfg(utils.RootPath())), ginI18n.WithGetLngHandle(
			func(context *gin.Context, defaultLng string) string {
				lng := context.Request.Header.Get("think-lang")
				if lng == "" {
					return defaultLng
				}
				return lng
			},
		)),
	)

	rootDir := utils.RootPath()
	lockPath := filepath.Join(rootDir, "public", installRouter.LockFileName)
	router.Use(middleware.InstallGuard(lockPath))

	// 静态资源与前台入口（不经渠道注册器）。
	router.Static("/assets", filepath.Join(rootDir, "public/assets"))
	router.Static("/static", filepath.Join(rootDir, "public/static"))
	router.Static("/storage/default", filepath.Join(rootDir, "public/storage/default"))
	registerRootRoute(router, rootDir)
	router.StaticFile("/favicon.ico", filepath.Join(rootDir, "public/favicon.ico"))

	// 渠道注册器：admin 负责 /admin/*，api 负责 /api/*，install 负责
	// /install 与 /api/install/*。
	adminR.Register(router)
	apiR.Register(router)
	installR.Register(router)

	admin.CollectRoutes(router)

	return router
}

func registerRootRoute(router *gin.Engine, rootDir string) {
	indexPath := filepath.Join(rootDir, "public", "index.html")
	// InstallHandler.isInstallComplete also checks the completion marker, while
	// this route treats any existing rootDir/public/install.lock as installed.
	lockPath := filepath.Join(rootDir, "public", installRouter.LockFileName)
	serveIndex := func(c *gin.Context) {
		if _, err := os.Stat(lockPath); err != nil {
			c.Redirect(http.StatusFound, "/install")
			return
		}
		c.File(indexPath)
	}

	router.GET("/", serveIndex)
	router.HEAD("/", serveIndex)
}

func registerHealthRoute(router *gin.Engine) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
