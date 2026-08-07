package router

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"

	adminRouter "buildadmin-go/internal/admin/router"
	apiRouter "buildadmin-go/internal/api/router"
	"buildadmin-go/internal/common/country"
	"buildadmin-go/internal/i18n"
	"buildadmin-go/internal/middleware"
	"buildadmin-go/internal/pkg/route"
	"buildadmin-go/internal/pkg/util"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitRouter 是根装配件：创建 gin.Engine、挂载全局中间件（Cors/Record/
// Logger/CustomRecovery/i18n）与静态资源，然后调用 admin、api 两个渠道
// 注册器完成挂载。各渠道的模块 registrar 已由渠道包自行聚合注入
// （internal/admin/router.ProvideRegistrars、internal/api/router.ProvideRegistrars），
// 根装配件不再按 Group() 分派。
// countrySvc 提供前台默认语言（country_language 第一条）供 /api/* 无
// think-lang header 时分流取用。
func InitRouter(
	loggerWriter *lumberjack.Logger,
	adminR *adminRouter.AdminRouter,
	apiR *apiRouter.ApiRouter,
	countrySvc *country.Service,
) *gin.Engine {
	router := gin.New()
	registerHealthRoute(router)

	// 跨域处理。Record 是 admin 渠道的中间件，但按既有语义挂在全局链上。
	router.Use(middleware.Cors(), adminR.RecordHandler())
	router.Use(
		gin.Logger(),
		middleware.CustomRecovery(loggerWriter),
		//开启多语言：按渠道分流（think-lang header 优先，/api/* 无 header
		//取前台默认语言，/admin/* 及其它默认中文），返回 ginI18n pack key。
		ginI18n.Localize(ginI18n.WithBundle(i18n.NewBundleCfg()), ginI18n.WithGetLngHandle(
			func(context *gin.Context, defaultLng string) string {
				return resolveRequestLang(context, defaultLng, countrySvc.DefaultLan)
			},
		)),
	)

	rootDir := util.RootPath()

	// 静态资源与前台入口（不经渠道注册器）。
	router.Static("/assets", filepath.Join(rootDir, "public/assets"))
	router.Static("/static", filepath.Join(rootDir, "public/static"))
	router.Static("/storage/default", filepath.Join(rootDir, "public/storage/default"))
	registerRootRoute(router, rootDir)
	router.StaticFile("/favicon.ico", filepath.Join(rootDir, "public/favicon.ico"))

	// 渠道注册器：admin 负责 /admin/*，api 负责 /api/*。
	adminR.Register(router)
	apiR.Register(router)

	route.CollectRoutes(router)

	return router
}

func registerRootRoute(router *gin.Engine, rootDir string) {
	indexPath := filepath.Join(rootDir, "public", "index.html")
	serveIndex := func(c *gin.Context) {
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

// resolveRequestLang 按渠道决定当前请求的语言（ginI18n WithGetLngHandle 回调，
// defaultLng 为 bundle 默认语言，与 ginI18n handler 签名保持一致）：
//   - think-lang header 优先（任何渠道），经 NormalizeLang 规范化到 pack key；
//   - /api/* 无 header 时取前台默认语言（country_language 第一条，经
//     NormalizeLang 规范化），失败或空值兜底 zh；
//   - /admin/* 及其它路径默认 zh（后端 pack key）。
func resolveRequestLang(c *gin.Context, defaultLng string, portalDefaultLang func(ctx context.Context) (string, error)) string {
	if lng := c.Request.Header.Get("think-lang"); lng != "" {
		return i18n.NormalizeLang(lng)
	}
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		if lan, err := portalDefaultLang(c.Request.Context()); err == nil && lan != "" {
			return i18n.NormalizeLang(lan)
		}
		return "zh"
	}
	return "zh" // /admin/* 及其它默认中文
}
