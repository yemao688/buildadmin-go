package router

import (
	"encoding/json"
	admin "go-build-admin/app/admin/handler"
	api "go-build-admin/app/api/handler"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
)

func InitRouter(
	adminHandler *admin.AdminHandler,
	adminLogHandler *admin.AdminLogHandler,
	testBuildHandler *admin.TestBuildHandler,
	indexHandler *admin.IndexHandler,
	dashboardHandler *admin.DashboardHandler,

	apiAccountHandler *api.AccountHandler,
	apiAjaxHandler *api.AjaxHandler,
	apiCommonHandler *api.CommonHandler,
	apiEmsHandler *api.EmsHandler,
	apiIndexHandler *api.IndexHandler,
	apiInstallHandler *api.InstallHandler,
	apiUserHandler *api.UserHandler,

) *gin.Engine {
	router := gin.New()
	router.Use(
		// gin.Logger(),
		// middleware.CustomRecovery(loggerWriter),
		//开启多语言
		ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
			RootPath:         "conf/localize",
			AcceptLanguage:   []language.Tag{language.Chinese, language.TraditionalChinese, language.English},
			DefaultLanguage:  language.Chinese,
			UnmarshalFunc:    json.Unmarshal,
			FormatBundleFile: "json",
		})),
	)

	// 引入admin路由
	adminRouter := router.Group("/admin/")
	adminRouter.GET("Index/index", indexHandler.Index)
	adminRouter.POST("Index/login", indexHandler.Login)
	adminRouter.POST("Index/logout", indexHandler.Logout)

	adminRouter.GET("Dashboard/index", dashboardHandler.Index)

	// adminRouter.GET("auth.Group/index", adminGroupHandler.Index)
	// adminRouter.POST("auth.Group/add", adminGroupHandler.Add)
	// adminRouter.POST("auth.Group/edit", adminGroupHandler.Edit)
	// adminRouter.POST("auth.Group/del", adminGroupHandler.Del)

	adminRouter.GET("auth.Admin/index", adminHandler.Index)
	adminRouter.POST("auth.Admin/add", adminHandler.Add)
	adminRouter.POST("auth.Admin/edit", adminHandler.Edit)
	adminRouter.POST("auth.Admin/del", adminHandler.Del)

	// adminRouter.GET("auth.Rule/index", adminRuleHandler.Index)
	// adminRouter.POST("auth.Rule/add", adminRuleHandler.Add)
	// adminRouter.POST("auth.Rule/edit", adminRuleHandler.Edit)
	// adminRouter.POST("auth.Rule/del", adminRuleHandler.Del)

	adminRouter.GET("auth.AdminLog/index", adminLogHandler.Index)

	// adminRouter.GET("user.User/index", userHandler.Index)
	// adminRouter.POST("user.User/add", userHandler.Add)
	// adminRouter.POST("user.User/edit", userHandler.Edit)
	// adminRouter.POST("user.User/del", userHandler.Del)

	// adminRouter.GET("user.Group/index", userGroupHandler.Index)
	// adminRouter.POST("user.Group/add", userGroupHandler.Add)
	// adminRouter.POST("user.Group/edit", userGroupHandler.Edit)
	// adminRouter.POST("user.Group/del", userGroupHandler.Del)

	// adminRouter.GET("user.Rule/index", userRuleHandler.Index)
	// adminRouter.POST("user.Rule/add", userRuleHandler.Add)
	// adminRouter.POST("user.Rule/edit", userRuleHandler.Edit)
	// adminRouter.POST("user.Rule/del", userRuleHandler.Del)

	// adminRouter.GET("user.MoneyLog/del", userMoneyLogHandler.Index)
	// adminRouter.POST("user.MoneyLog/add", userMoneyLogHandler.Add)

	// adminRouter.GET("user.ScoreLog/del", userScoreLogHandler.Index)
	// adminRouter.POST("user.ScoreLog/add", userScoreLogHandler.Add)

	// adminRouter.POST("routine.Config/index", configHandler.Index)

	// adminRouter.GET("routine.Attachment/index", attachmentHandler.Index)
	// adminRouter.POST("routine.Attachment/add", attachmentHandler.Add)
	// adminRouter.POST("routine.Attachment/edit", attachmentHandler.Edit)
	// adminRouter.POST("routine.Attachment/del", attachmentHandler.Del)

	// adminRouter.GET("routine.AdminInfo/index", adminInfoHandler.Index)
	// adminRouter.POST("routine.AdminInfo/add", adminInfoHandler.Add)

	// adminRouter.GET("security.DataRecycleLog/index", dataRecycleLogHandler.Index)
	// adminRouter.POST("security.DataRecycleLog/add", dataRecycleLogHandler.Add)
	// adminRouter.POST("security.DataRecycleLog/edit", dataRecycleLogHandler.Edit)
	// adminRouter.POST("security.DataRecycleLog/del", dataRecycleLogHandler.Del)

	// adminRouter.GET("security.DataRecycle/index", dataRecycleHandler.Index)
	// adminRouter.POST("security.DataRecycle/add", dataRecycleHandler.Add)
	// adminRouter.POST("security.DataRecycle/edit", dataRecycleHandler.Edit)
	// adminRouter.POST("security.DataRecycle/del", dataRecycleHandler.Del)

	// adminRouter.GET("security.SensitiveDataLog/index", sensitiveDataLogHandler.Index)
	// adminRouter.POST("security.SensitiveDataLog/add", sensitiveDataLogHandler.Add)
	// adminRouter.POST("security.SensitiveDataLog/edit", sensitiveDataLogHandler.Edit)
	// adminRouter.POST("security.SensitiveDataLog/del", sensitiveDataLogHandler.Del)

	// adminRouter.GET("security.SensitiveData/index", sensitiveDataHandler.Index)
	// adminRouter.POST("security.SensitiveData/add", sensitiveDataHandler.Add)
	// adminRouter.POST("security.SensitiveData/edit", sensitiveDataHandler.Edit)
	// adminRouter.POST("security.SensitiveData/del", sensitiveDataHandler.Del)

	// adminRouter.GET("crud.Crud/databaseList", crudHandler.DatabaseList)

	adminRouter.GET("testBuild/index", testBuildHandler.Index)
	adminRouter.POST("testBuild/add", testBuildHandler.Add)
	adminRouter.POST("testBuild/edit", testBuildHandler.Edit)
	adminRouter.POST("testBuild/del", testBuildHandler.Del)

	// 引入api接口路由
	apiRouter := router.Group("/api/")
	apiRouter.POST("account/overview", apiAccountHandler.Overview)
	apiRouter.POST("account/Profile", apiAccountHandler.Profile)
	apiRouter.POST("account/Verification", apiAccountHandler.Verification)
	apiRouter.POST("account/ChangeBind", apiAccountHandler.ChangeBind)
	apiRouter.POST("account/ChangePassword", apiAccountHandler.ChangePassword)
	apiRouter.POST("account/Integral", apiAccountHandler.Integral)
	apiRouter.POST("account/Balance", apiAccountHandler.Balance)
	apiRouter.POST("account/RetrievePassword", apiAccountHandler.RetrievePassword)

	apiRouter.POST("ajax/upload", apiAjaxHandler.Upload)
	apiRouter.POST("ajax/area", apiAjaxHandler.Area)
	apiRouter.POST("ajax/buildSuffixSvg", apiAjaxHandler.BuildSuffixSvg)

	apiRouter.GET("common/captcha", apiCommonHandler.Captcha)
	apiRouter.GET("common/clickCaptcha", apiCommonHandler.ClickCaptcha)
	apiRouter.POST("common/checkClickCaptcha", apiCommonHandler.CheckClickCaptcha)
	apiRouter.POST("common/refreshToken", apiCommonHandler.RefreshToken)

	apiRouter.POST("ems/send", apiEmsHandler.Send)
	apiRouter.POST("index/index", apiIndexHandler.Index)
	apiRouter.POST("install/changePackageManager", apiInstallHandler.ChangePackageManager)
	apiRouter.POST("install/envBaseCheck", apiInstallHandler.EnvBaseCheck)
	apiRouter.POST("install/envNpmCheck", apiInstallHandler.EnvNpmCheck)
	apiRouter.POST("install/testDatabase", apiInstallHandler.TestDatabase)
	apiRouter.POST("install/baseConfig", apiInstallHandler.BaseConfig)
	apiRouter.POST("install/commandExecComplete", apiInstallHandler.CommandExecComplete)
	apiRouter.POST("install/manualInstall", apiInstallHandler.ManualInstall)
	apiRouter.POST("install/mvDist", apiInstallHandler.MvDist)

	apiRouter.POST("user/checkIn", apiUserHandler.CheckIn)
	apiRouter.POST("user/logout", apiUserHandler.Logout)
	return router
}
