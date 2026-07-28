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

	adminHandler *admin.AdminHandler,
	adminInfoHandler *admin.AdminInfoHandler,
	adminLogHandler *admin.AdminLogHandler,
	indexHandler *admin.IndexHandler,
	dashboardHandler *admin.DashboardHandler,
	userHandler *admin.UserHandler,
	userMoneyLogHandler *admin.UserMoneyLogHandler,
	userScoreLogHandler *admin.UserScoreLogHandler,
	crudHandler *admin.CrudHandler,

	dataRecycleHandler *admin.DataRecycleHandler,
	dataRecycleLogHandler *admin.DataRecycleLogHandler,
	sensitiveDataHandler *admin.SensitiveDataHandler,
	sensitiveDataLogHandler *admin.SensitiveDataLogHandler,
	ajaxHandler *admin.AjaxHandler,

	apiAccountHandler *api.AccountHandler,
	apiAjaxHandler *api.AjaxHandler,
	apiCommonHandler *api.CommonHandler,
	apiEmsHandler *api.EmsHandler,
	apiIndexHandler *api.IndexHandler,
	apiInstallHandler *api.InstallHandler,
	apiUserHandler *api.UserHandler,
	apiDemoHandler *api.DemoHandler,

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
	for _, capability := range []middleware.AtomicRoute{
		{Route: "auth/admin", Action: "add", Method: http.MethodPost},
		{Route: "auth/admin", Action: "edit", Method: http.MethodPost},
		{Route: "auth/admin", Action: "del", Method: http.MethodDelete},
		{Route: "user/user", Action: "add", Method: http.MethodPost},
		{Route: "user/user", Action: "edit", Method: http.MethodPost},
		{Route: "user/user", Action: "del", Method: http.MethodDelete},
		{Route: "security/datarecycle", Action: "add", Method: http.MethodPost},
		{Route: "security/datarecycle", Action: "edit", Method: http.MethodPost},
		{Route: "security/datarecycle", Action: "del", Method: http.MethodDelete},
		{Route: "security/datarecyclelog", Action: "restore", Method: http.MethodPost},
		{Route: "security/datarecyclelog", Action: "del", Method: http.MethodDelete},
		{Route: "security/sensitivedata", Action: "add", Method: http.MethodPost},
		{Route: "security/sensitivedata", Action: "edit", Method: http.MethodPost},
		{Route: "security/sensitivedata", Action: "del", Method: http.MethodDelete},
		{Route: "security/sensitivedatalog", Action: "rollback", Method: http.MethodPost},
		{Route: "security/sensitivedatalog", Action: "del", Method: http.MethodDelete},
	} {
		middleware.RegisterAtomicRoute(capability)
	}
	adminRouter.GET("Index/index", indexHandler.Index)
	adminRouter.POST("Index/logout", indexHandler.Logout)

	adminRouter.GET("Dashboard/index", dashboardHandler.Index)

	adminRouter.GET("auth.Admin/index", adminHandler.Index)
	adminRouter.POST("auth.Admin/add", adminHandler.Add)
	adminRouter.GET("auth.Admin/edit", adminHandler.One)
	adminRouter.POST("auth.Admin/edit", adminHandler.Edit)
	adminRouter.DELETE("auth.Admin/del", adminHandler.Del)

	registerAdminLogRoutes(adminRouter, adminLogHandler)

	adminRouter.GET("user.User/index", userHandler.Index)
	adminRouter.POST("user.User/add", userHandler.Add)
	adminRouter.GET("user.User/edit", userHandler.One)
	adminRouter.POST("user.User/edit", userHandler.Edit)
	adminRouter.DELETE("user.User/del", userHandler.Del)

	adminRouter.GET("user.MoneyLog/index", userMoneyLogHandler.Index)
	adminRouter.GET("user.MoneyLog/add", userHandler.One)
	adminRouter.POST("user.MoneyLog/add", userMoneyLogHandler.Add)

	adminRouter.GET("user.ScoreLog/index", userScoreLogHandler.Index)
	adminRouter.GET("user.ScoreLog/add", userHandler.One)
	adminRouter.POST("user.ScoreLog/add", userScoreLogHandler.Add)

	adminRouter.GET("routine.AdminInfo/index", adminInfoHandler.Index)
	adminRouter.POST("routine.AdminInfo/edit", adminInfoHandler.Edit)

	adminRouter.GET("security.DataRecycleLog/index", dataRecycleLogHandler.Index)
	adminRouter.GET("security.DataRecycleLog/info", dataRecycleLogHandler.Info)
	adminRouter.POST("security.DataRecycleLog/restore", dataRecycleLogHandler.Restore)
	adminRouter.DELETE("security.DataRecycleLog/del", dataRecycleLogHandler.Del)

	adminRouter.GET("security.DataRecycle/index", dataRecycleHandler.Index)
	adminRouter.GET("security.DataRecycle/add", dataRecycleHandler.Add)
	adminRouter.POST("security.DataRecycle/add", dataRecycleHandler.Add)
	adminRouter.GET("security.DataRecycle/edit", dataRecycleHandler.One)
	adminRouter.POST("security.DataRecycle/edit", dataRecycleHandler.Edit)
	adminRouter.DELETE("security.DataRecycle/del", dataRecycleHandler.Del)

	adminRouter.GET("security.SensitiveDataLog/index", sensitiveDataLogHandler.Index)
	adminRouter.GET("security.SensitiveDataLog/info", sensitiveDataLogHandler.Info)
	adminRouter.POST("security.SensitiveDataLog/rollback", sensitiveDataLogHandler.Rollback)
	adminRouter.DELETE("security.SensitiveDataLog/del", sensitiveDataLogHandler.Del)

	adminRouter.GET("security.SensitiveData/index", sensitiveDataHandler.Index)
	adminRouter.GET("security.SensitiveData/add", sensitiveDataHandler.Add)
	adminRouter.POST("security.SensitiveData/add", sensitiveDataHandler.Add)
	adminRouter.GET("security.SensitiveData/edit", sensitiveDataHandler.One)
	adminRouter.POST("security.SensitiveData/edit", sensitiveDataHandler.Edit)
	adminRouter.DELETE("security.SensitiveData/del", sensitiveDataHandler.Del)

	adminRouter.GET("crud.Crud/databaseList", crudHandler.DatabaseList)
	adminRouter.GET("crud.Crud/checkCrudLog", crudHandler.CheckCrudLog)
	adminRouter.POST("crud.Crud/parseFieldData", crudHandler.ParseFieldData)
	adminRouter.GET("crud.Crud/getFileData", crudHandler.GetFileData)
	adminRouter.POST("crud.Crud/generateCheck", crudHandler.GenerateCheck)
	adminRouter.POST("crud.Crud/generate", crudHandler.Generate)
	adminRouter.POST("crud.Crud/logStart", crudHandler.LogStart)
	adminRouter.POST("crud.Crud/delete", crudHandler.Delete)

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
	registerPublicAccountRoutes(router, apiAccountHandler.RetrievePassword)
	router.POST("/api/ajax/area", apiAjaxHandler.Area)
	router.POST("/api/ajax/buildSuffixSvg", apiAjaxHandler.BuildSuffixSvg)
	router.POST("/api/Ems/send", apiEmsHandler.Send)
	router.GET("/api/index/index", apiIndexHandler.Index)
	router.GET("/api/user/checkIn", apiUserHandler.CheckIn)
	router.POST("/api/user/checkIn", apiUserHandler.CheckIn)

	router.GET("/api/common/captcha", apiCommonHandler.Captcha)
	router.GET("/api/common/clickCaptcha", apiCommonHandler.ClickCaptcha)
	router.POST("/api/common/checkClickCaptcha", apiCommonHandler.CheckClickCaptcha)
	router.POST("/api/common/refreshToken", apiCommonHandler.RefreshToken)

	router.POST("/api/demo/index", apiDemoHandler.Index)

	// 引入api接口路由
	apiRouter := router.Group("/api/").Use(userLoginM.Handler())
	apiRouter.GET("account/overview", apiAccountHandler.Overview)
	apiRouter.GET("account/profile", apiAccountHandler.Profile)
	apiRouter.POST("account/profile", apiAccountHandler.Profile)
	apiRouter.POST("account/verification", apiAccountHandler.Verification)
	apiRouter.POST("account/changeBind", apiAccountHandler.ChangeBind)
	apiRouter.POST("account/changePassword", apiAccountHandler.ChangePassword)
	apiRouter.GET("account/integral", apiAccountHandler.Integral)
	apiRouter.GET("account/balance", apiAccountHandler.Balance)

	apiRouter.POST("ajax/upload", apiAjaxHandler.Upload)
	apiRouter.POST("Alioss/callback", apiAjaxHandler.AliossCallback)
	apiRouter.POST("user/logout", apiUserHandler.Logout)

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
			routes = apiRouter
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

func registerAdminLogRoutes(adminRouter gin.IRoutes, handler *admin.AdminLogHandler) {
	adminRouter.GET("auth.AdminLog/index", handler.Index)
	adminRouter.DELETE("auth.AdminLog/del", handler.Del)
}

func registerPublicAccountRoutes(router *gin.Engine, accountHandler gin.HandlerFunc) {
	router.POST("/api/account/retrievePassword", accountHandler)
}

func registerHealthRoute(router *gin.Engine) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
