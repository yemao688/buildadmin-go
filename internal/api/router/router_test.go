package router

import (
	"net/http"
	"strings"
	"testing"

	api "buildadmin-go/internal/api/handler"
	apiMiddleware "buildadmin-go/internal/api/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicUserAuthenticationRoutesUseFrontendPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
	router := gin.New()
	apiRoutes := newAPIRouteSet(router, router.Group("/api/"))
	NewUserRegistrar(&api.UserHandler{}).Register(apiRoutes)

	want := map[string]bool{
		"/api/user/login":    false,
		"/api/user/register": false,
		"/api/user/logout":   false,
	}
	for _, route := range router.Routes() {
		if route.Method == http.MethodPost {
			if _, ok := want[route.Path]; ok {
				want[route.Path] = true
			}
		}
	}
	for path, found := range want {
		require.True(t, found, "public user route %s is not registered", path)
	}
}

// 保证 api_routes.go 的 public 名单不是
// 一份会漂移的愿望清单：名单里的每个 method+path 都必须真的注册到根引擎，
// 否则免登录声明会静默失效（新增端点忘了注册）或留下死条目。
func TestPublicRouteSetHasNoOrphans(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
	engine := gin.New()
	rawSet := newAPIRouteSet(engine, engine.Group("/api/"))
	set, ok := rawSet.(*apiRouteSet)
	require.True(t, ok, "newAPIRouteSet must return *apiRouteSet")
	NewUserRegistrar(&api.UserHandler{}).Register(set)
	NewCommonRegistrar(&api.CommonHandler{}).Register(set)
	NewIndexRegistrar(&api.IndexHandler{}).Register(set)

	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for key := range set.public {
		method, rel, ok := strings.Cut(key, " ")
		require.True(t, ok, "malformed public route key %q", key)
		path := "/api/" + rel
		require.True(t, registered[method+" "+path], "public route %s %s is declared but not registered", method, path)
	}
}

func TestApiRouterMountsAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
	engine := gin.New()
	NewApiRouter(ApiRouterDeps{
		UserLoginM: &apiMiddleware.UserLogin{},
		Registrars: []Registrar{
			NewUserRegistrar(&api.UserHandler{}),
			NewCommonRegistrar(&api.CommonHandler{}),
			NewIndexRegistrar(&api.IndexHandler{}),
		},
	}).Register(engine)

	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	// 注册器声明的每一条路由都必须出现在引擎上，且不存在多余/重复路由：
	// 路由集合与注册器声明一一对应。
	want := []string{
		"POST /api/user/login",
		"POST /api/user/register",
		"POST /api/user/logout",
		"GET /api/common/captcha",
		"GET /api/common/clickCaptcha",
		"POST /api/common/checkClickCaptcha",
		"POST /api/common/refreshToken",
		"POST /api/ajax/upload",
		"POST /api/Alioss/callback",
		"GET /api/index/index",
	}
	require.Len(t, engine.Routes(), len(want))
	for _, route := range want {
		require.True(t, registered[route], "route %s is not registered", route)
	}
}
