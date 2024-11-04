package router

import (
	"encoding/json"
	admin "go-build-admin/app/admin/handler"
	api "go-build-admin/app/api/handler"
	"go-build-admin/app/middleware"
	"go-build-admin/utils"
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
	dataLimitM *middleware.DataLimit,
	recordM *middleware.Record,

	adminHandler *admin.AdminHandler,
	adminInfoHandler *admin.AdminInfoHandler,
	adminGroupHandler *admin.AdminGroupHandler,
	adminRuleHandler *admin.AdminRuleHandler,
	adminLogHandler *admin.AdminLogHandler,
	testBuildHandler *admin.TestBuildHandler,
	indexHandler *admin.IndexHandler,
	dashboardHandler *admin.DashboardHandler,
	userHandler *admin.UserHandler,
	userGroupHandler *admin.UserGroupHandler,
	userRuleHandler *admin.UserRuleHandler,
	userMoneyLogHandler *admin.UserMoneyLogHandler,
	userScoreLogHandler *admin.UserScoreLogHandler,
	attachmentHandler *admin.AttachmentHandler,
	crudHandler *admin.CrudHandler,
	crudLogHandler *admin.CrudLogHandler,
	configHandler *admin.ConfigHandler,
	moduleHandler *admin.ModuleHandler,

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

) *gin.Engine {
	router := gin.New()
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

	router.Static("/static", filepath.Join(rootDir, "static"))
	router.Static("/storage/default", filepath.Join(rootDir, "storage/default"))

	router.GET("/admin/Index/login", indexHandler.Login)
	router.POST("/admin/Index/login", indexHandler.Login)
	router.GET("/admin/ajax/buildSuffixSvg", ajaxHandler.BuildSuffixSvg)
	router.GET("/admin/ajax/terminal", ajaxHandler.Terminal)

	// 引入admin路由
	adminRouter := router.Group("/admin/").Use(loginM.Handler(), securityM.Handler())
	adminRouter.GET("Index/index", indexHandler.Index)
	adminRouter.POST("Index/logout", indexHandler.Logout)

	adminRouter.GET("Dashboard/index", dashboardHandler.Index)

	adminRouter.GET("auth.Group/index", adminGroupHandler.Index)
	adminRouter.POST("auth.Group/add", adminGroupHandler.Add)
	adminRouter.GET("auth.Group/edit", adminGroupHandler.One)
	adminRouter.POST("auth.Group/edit", adminGroupHandler.Edit)
	adminRouter.DELETE("auth.Group/del", adminGroupHandler.Del)

	adminRouter.GET("auth.Admin/index", dataLimitM.Handler("allAuthAndOthers"), adminHandler.Index)
	adminRouter.POST("auth.Admin/add", adminHandler.Add)
	adminRouter.GET("auth.Admin/edit", adminHandler.One)
	adminRouter.POST("auth.Admin/edit", adminHandler.Edit)
	adminRouter.DELETE("auth.Admin/del", adminHandler.Del)

	adminRouter.GET("auth.Rule/index", adminRuleHandler.Index)
	adminRouter.POST("auth.Rule/add", adminRuleHandler.Add)
	adminRouter.GET("auth.Rule/edit", adminRuleHandler.One)
	adminRouter.POST("auth.Rule/edit", adminRuleHandler.Edit)
	adminRouter.DELETE("auth.Rule/del", adminRuleHandler.Del)
	adminRouter.POST("auth.Rule/sortable", adminRuleHandler.Sortable)

	adminRouter.GET("auth.AdminLog/index", adminLogHandler.Index)

	adminRouter.GET("user.User/index", userHandler.Index)
	adminRouter.POST("user.User/add", userHandler.Add)
	adminRouter.GET("user.User/edit", userHandler.One)
	adminRouter.POST("user.User/edit", userHandler.Edit)
	adminRouter.DELETE("user.User/del", userHandler.Del)

	adminRouter.GET("user.Group/index", userGroupHandler.Index)
	adminRouter.POST("user.Group/add", userGroupHandler.Add)
	adminRouter.GET("user.Group/edit", userGroupHandler.One)
	adminRouter.POST("user.Group/edit", userGroupHandler.Edit)
	adminRouter.DELETE("user.Group/del", userGroupHandler.Del)

	adminRouter.GET("user.Rule/index", userRuleHandler.Index)
	adminRouter.POST("user.Rule/add", userRuleHandler.Add)
	adminRouter.GET("user.Rule/edit", userRuleHandler.One)
	adminRouter.POST("user.Rule/edit", userRuleHandler.Edit)
	adminRouter.DELETE("user.Rule/del", userRuleHandler.Del)
	adminRouter.POST("user.Rule/sortable", userRuleHandler.Sortable)

	adminRouter.GET("user.MoneyLog/index", userMoneyLogHandler.Index)
	adminRouter.GET("user.MoneyLog/add", userHandler.One)
	adminRouter.POST("user.MoneyLog/add", userMoneyLogHandler.Add)

	adminRouter.GET("user.ScoreLog/index", userScoreLogHandler.Index)
	adminRouter.GET("user.ScoreLog/add", userHandler.One)
	adminRouter.POST("user.ScoreLog/add", userScoreLogHandler.Add)

	adminRouter.GET("routine.Config/index", configHandler.Index)
	adminRouter.POST("routine.Config/add", configHandler.Add)
	adminRouter.POST("routine.Config/edit", configHandler.Edit)
	adminRouter.DELETE("routine.Config/del", configHandler.Del)
	adminRouter.POST("routine.Config/sendTestMail", configHandler.SendTestMail)

	adminRouter.GET("routine.Attachment/index", attachmentHandler.Index)
	adminRouter.GET("routine.Attachment/edit", attachmentHandler.One)
	adminRouter.POST("routine.Attachment/edit", attachmentHandler.Edit)
	adminRouter.DELETE("routine.Attachment/del", attachmentHandler.Del)

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

	adminRouter.GET("crud.Log/index", crudLogHandler.Index)

	adminRouter.GET("module/index", moduleHandler.Index)

	adminRouter.GET("ajax/area", ajaxHandler.Area)
	adminRouter.POST("ajax/upload", ajaxHandler.Upload)
	adminRouter.GET("ajax/getTablePk", ajaxHandler.GetTablePk)
	adminRouter.GET("ajax/getTableFieldList", ajaxHandler.GetTableFieldList)
	adminRouter.POST("ajax/clearCache", ajaxHandler.ClearCache)
	adminRouter.POST("ajax/changeTerminalConfig", ajaxHandler.ChangeTerminalConfig)

	adminRouter.GET("testBuild/index", testBuildHandler.Index)
	adminRouter.POST("testBuild/add", testBuildHandler.Add)
	adminRouter.GET("testBuild/edit", testBuildHandler.One)
	adminRouter.POST("testBuild/edit", testBuildHandler.Edit)
	adminRouter.DELETE("testBuild/del", testBuildHandler.Del)

	//-----------------------api 接口部分--------------------//
	router.POST("/api/account/RetrievePassword", apiAccountHandler.RetrievePassword)
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
	apiRouter.POST("user/logout", apiUserHandler.Logout)

	admin.CollectRoutes(router)

	return router
}
