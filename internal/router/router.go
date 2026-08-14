package router

import (
	"context"
	"net/http"
	"net/http/pprof"
	"path/filepath"
	"strings"

	adminRouter "buildadmin-go/internal/admin/router"
	apiRouter "buildadmin-go/internal/api/router"
	"buildadmin-go/internal/common/country"
	"buildadmin-go/internal/conf"
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
// think-lang header 时分流取用。config 提供跨域白名单等应用配置，nil 时
// 按空白名单处理（跨域仅自身 host 放行）。
func InitRouter(
	loggerWriter *lumberjack.Logger,
	config *conf.Configuration,
	adminR *adminRouter.AdminRouter,
	apiR *apiRouter.ApiRouter,
	countrySvc *country.Service,
) *gin.Engine {
	router := gin.New()
	registerHealthRoute(router)
	// pprof 调试端点仅在非 release 模式挂载，release 生产环境不暴露。
	if gin.Mode() != gin.ReleaseMode {
		registerPprofRoutes(router)
	}

	// 跨域处理。Record 是 admin 渠道的中间件，但按既有语义挂在全局链上。
	// config 为 nil 时按空白名单处理（Cors 空串白名单，自身 host 仍放行）。
	corsRequestDomain := ""
	if config != nil {
		corsRequestDomain = config.App.CorsRequestDomain
	}
	router.Use(middleware.Cors(corsRequestDomain), adminR.RecordHandler())
	// gin.Logger 仅在非 release 模式挂载：debug 开发逐请求打 stdout 有用，
	// release 去掉逐请求访问日志（SQL 日志由 gorm 配置控制，API 访问日志
	// 非本框架职责）。挂载顺序保持 Logger 在 CustomRecovery/i18n 之前。
	if gin.Mode() != gin.ReleaseMode {
		router.Use(gin.Logger())
	}
	router.Use(
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
	// 整个 storage 目录：上传 URL 前缀是 /storage/{topic}（savename 规则），
	// 挂载目录级而非 /storage/default，topic 变化（default/其它细目）无需改路由。
	router.Static("/storage", filepath.Join(rootDir, "public/storage"))
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

// registerPprofRoutes 挂载标准库 net/http/pprof 调试端点（非 release 模式）。
// 逐路径注册，路由集与 import _ "net/http/pprof" 在默认 mux 上注册的完全一致：
// Index 负责 /debug/pprof/ 索引页，cmdline/profile/symbol/trace 是独立函数，
// 六个命名 profile（allocs/block/goroutine/heap/mutex/threadcreate）经
// pprof.Handler 按名分发。不引入 gin-contrib/pprof 等第三方依赖。
func registerPprofRoutes(router *gin.Engine) {
	router.GET("/debug/pprof/", gin.WrapF(pprof.Index))
	router.GET("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	router.GET("/debug/pprof/profile", gin.WrapF(pprof.Profile))
	router.GET("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	router.GET("/debug/pprof/trace", gin.WrapF(pprof.Trace))
	router.GET("/debug/pprof/allocs", gin.WrapH(pprof.Handler("allocs")))
	router.GET("/debug/pprof/block", gin.WrapH(pprof.Handler("block")))
	router.GET("/debug/pprof/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	router.GET("/debug/pprof/heap", gin.WrapH(pprof.Handler("heap")))
	router.GET("/debug/pprof/mutex", gin.WrapH(pprof.Handler("mutex")))
	router.GET("/debug/pprof/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
}

// resolveRequestLang 按渠道决定当前请求的语言（ginI18n WithGetLngHandle 回调，
// defaultLng 为 bundle 默认语言，与 ginI18n handler 签名保持一致）：
//   - think-lang header 优先（任何渠道），经 NormalizeLang 规范化到 pack key；
//   - /api/* 无 header 时取前台默认语言（country_language 第一条，经
//     NormalizeLang 规范化），失败或空值兜底 zh；
//   - /admin/* 及其它路径默认 zh（后端 pack key）。
//
// 解析结果在首次调用时写入 gin context（i18n.SetLangToContext），同一请求内
// 后续翻译调用（ginI18n 每次 getMessage 都会回调本函数）直接读缓存，不再
// 重复解析或查库。同一请求内语言不会中途变化，缓存后行为与首次解析一致。
func resolveRequestLang(c *gin.Context, defaultLng string, portalDefaultLang func(ctx context.Context) (string, error)) string {
	if lang, ok := i18n.LangFromContext(c); ok {
		return lang
	}
	var lang string
	switch {
	case c.Request.Header.Get("think-lang") != "":
		lang = i18n.NormalizeLang(c.Request.Header.Get("think-lang"))
	case strings.HasPrefix(c.Request.URL.Path, "/api/"):
		if lan, err := portalDefaultLang(c.Request.Context()); err == nil && lan != "" {
			lang = i18n.NormalizeLang(lan)
		} else {
			lang = "zh"
		}
	default: // /admin/* 及其它默认中文
		lang = "zh"
	}
	i18n.SetLangToContext(c, lang)
	return lang
}
