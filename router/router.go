package router

import (
	"encoding/json"
	admin "go-build-admin/app/admin/handler"
	api "go-build-admin/app/api/handler"
	"go-build-admin/app/middleware"
	"go-build-admin/utils"
	"net/http"
	"path/filepath"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
	"gopkg.in/natefinch/lumberjack.v2"
)

func InitRouter(
	loggerWriter *lumberjack.Logger,
	loginM *middleware.Login,
	securityM *middleware.Security,
	userLoginM *middleware.UserLogin,
	recordM *middleware.Record,

	indexHandler *admin.IndexHandler,

	ajaxHandler *admin.AjaxHandler,

	apiInstallHandler *api.InstallHandler,

	registrars []RouteRegistrar,
) *gin.Engine {
	router := gin.New()
	registerHealthRoute(router)

	// 跨域处理
	router.Use(middleware.Cors(), recordM.Handler())
	router.Use(
		gin.Logger(),
		middleware.CustomRecovery(loggerWriter),
		//开启多语言
		ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
			RootPath:         utils.RootPath() + "/conf/localize",
			AcceptLanguage:   []language.Tag{language.Chinese, language.TraditionalChinese, language.English},
			DefaultLanguage:  language.Chinese,
			UnmarshalFunc:    json.Unmarshal,
			FormatBundleFile: "json",
		}), ginI18n.WithGetLngHandle(
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
	router.Static("/install", filepath.Join(rootDir, "static/install"))
	router.POST("/api/install/changePackageManager", apiInstallHandler.ChangePackageManager)
	router.GET("/api/install/envBaseCheck", apiInstallHandler.EnvBaseCheck)
	router.POST("/api/install/envNpmCheck", apiInstallHandler.EnvNpmCheck)
	router.GET("/api/install/terminal", apiInstallHandler.Terminal)
	router.GET("/api/install/baseConfig", apiInstallHandler.BaseConfig)
	router.POST("/api/install/baseConfig", apiInstallHandler.BaseConfig)
	router.POST("/api/install/testDatabase", apiInstallHandler.TestDatabase)
	router.POST("/api/install/commandExecComplete", apiInstallHandler.CommandExecComplete)
	router.POST("/api/install/manualInstall", apiInstallHandler.ManualInstall)
	router.POST("/api/install/mvDist", apiInstallHandler.MvDist)

	router.GET("/admin/Index/login", indexHandler.Login)
	router.POST("/admin/Index/login", indexHandler.Login)
	router.GET("/admin/ajax/buildSuffixSvg", ajaxHandler.BuildSuffixSvg)
	router.GET("/admin/ajax/terminal", ajaxHandler.Terminal)

	// 引入admin路由
	adminRouter := router.Group("/admin/").Use(loginM.Handler(), securityM.Handler())
	adminRouter.GET("Index/index", indexHandler.Index)
	adminRouter.POST("Index/logout", indexHandler.Logout)

	adminRouter.GET("ajax/area", ajaxHandler.Area)
	adminRouter.POST("ajax/upload", ajaxHandler.Upload)
	adminRouter.POST("Alioss/callback", ajaxHandler.AliossCallback)
	adminRouter.GET("ajax/getTablePk", ajaxHandler.GetTablePk)
	adminRouter.GET("ajax/getTableList", ajaxHandler.GetTableList)
	adminRouter.GET("ajax/getTableFieldList", ajaxHandler.GetTableFieldList)
	adminRouter.GET("ajax/getDatabaseConnectionList", ajaxHandler.GetDatabaseConnectionList)
	adminRouter.POST("ajax/clearCache", ajaxHandler.ClearCache)
	adminRouter.POST("ajax/changeTerminalConfig", ajaxHandler.ChangeTerminalConfig)

	//-----------------------api 接口部分--------------------//
	// 引入api接口路由
	apiRouter := router.Group("/api/").Use(userLoginM.Handler())

	router.Static("/assets", filepath.Join(rootDir, "static/assets"))
	router.Static("/static", filepath.Join(rootDir, "static"))
	router.Static("/storage/default", filepath.Join(rootDir, "storage/default"))
	router.StaticFile("/", filepath.Join(rootDir, "static/index.html"))

	for _, registrar := range registrars {
		for _, capability := range registrar.Capabilities() {
			middleware.RegisterAtomicRoute(capability)
		}
	}
	for _, registrar := range registrars {
		var routes gin.IRoutes
		switch registrar.Group() {
		case "admin":
			routes = adminRouter
		case "api":
			routes = newAPIRouteSet(router, apiRouter)
		case "root":
			routes = router
		default:
			panic("unknown route registrar group: " + registrar.Group())
		}
		registrar.Register(routes)
	}

	admin.CollectRoutes(router)

	return router
}

func registerHealthRoute(router *gin.Engine) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
