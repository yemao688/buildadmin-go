package router

import (
	"net/http"
	"reflect"
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
//
// 注册器统一取自生产聚合 ProvideRegistrars（而非在此逐个 new）：业务 fork
// 必须把业务 registrar 加进 ProvideRegistrars 才能完成生产挂载，本检查因此
// 自动覆盖业务公共端点。实参不写死——按函数自身签名反射构造零值 handler，
// 否则下游每追加一个 registrar 参数都会让本框架测试编译失败。
// 绕过该聚合自行拼注册器列表，等于同时绕过生产挂载路径，不在本测试承诺范围内。
func TestPublicRouteSetHasNoOrphans(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
	engine := gin.New()
	rawSet := newAPIRouteSet(engine, engine.Group("/api/"))
	set, ok := rawSet.(*apiRouteSet)
	require.True(t, ok, "newAPIRouteSet must return *apiRouteSet")
	registrars := buildTestRegistrars(t, ProvideRegistrars)
	require.NotEmpty(t, registrars, "ProvideRegistrars must expose the api module registrars")
	for _, registrar := range registrars {
		registrar.Register(set)
	}

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

// buildTestRegistrars 按 provider 自身签名反射构造零值 handler 并调用，
// 让注册面始终等于生产聚合面：业务 fork 在 ProvideRegistrars 锚点追加 registrar
// 参数后，本框架测试不需要跟着改实参（写死实参会直接编译失败）。
// Register 只登记 method+path，不解引用 handler 字段，零值 handler 足够。
func buildTestRegistrars(t *testing.T, provider any) []Registrar {
	t.Helper()
	fn := reflect.ValueOf(provider)
	require.Equal(t, reflect.Func, fn.Kind(), "registrar provider must be a function")
	inputs := make([]reflect.Value, fn.Type().NumIn())
	for i := range inputs {
		paramType := fn.Type().In(i)
		require.Equal(t, reflect.Pointer, paramType.Kind(), "ProvideRegistrars parameter %d must be a handler pointer", i)
		inputs[i] = reflect.New(paramType.Elem())
	}
	results := fn.Call(inputs)
	require.Len(t, results, 1, "ProvideRegistrars must return the registrar slice")
	registrars, ok := results[0].Interface().([]Registrar)
	require.True(t, ok, "ProvideRegistrars must return []Registrar")
	return registrars
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
