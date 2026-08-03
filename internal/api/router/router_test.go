package router

import (
	"net/http"
	"testing"

	api "buildadmin-go/internal/api/handler"
	apiMiddleware "buildadmin-go/internal/api/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPublicUserAuthenticationRoutesUseFrontendPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
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

func TestApiRouterMountsAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewApiRouter(ApiRouterDeps{
		UserLoginM: &apiMiddleware.UserLogin{},
		Registrars: []Registrar{
			NewUserRegistrar(&api.UserHandler{}),
			NewCommonRegistrar(&api.CommonHandler{}),
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
	}
	require.Len(t, engine.Routes(), len(want))
	for _, route := range want {
		require.True(t, registered[route], "route %s is not registered", route)
	}
}
