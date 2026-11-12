package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	admin "buildadmin-go/internal/admin/handler"
	adminMiddleware "buildadmin-go/internal/admin/middleware"
	"buildadmin-go/internal/middleware"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestAdminLogDeleteRoute(t *testing.T) {
	engine := gin.New()
	NewAdminRouter(AdminRouterDeps{
		LoginM:         &adminMiddleware.Login{},
		AuthorizationM: &adminMiddleware.Authorization{},
		SecurityM:      &adminMiddleware.Security{},
		RecordM:        &adminMiddleware.Record{},
		IndexHandler:   &admin.IndexHandler{},
		AjaxHandler:    &admin.AjaxHandler{},
		Registrars: []Registrar{
			NewAdminLogRegistrar(&admin.AdminLogHandler{}),
		},
	}).Register(engine)

	found := false
	for _, route := range engine.Routes() {
		if route.Method == http.MethodDelete && route.Path == "/admin/auth.AdminLog/del" {
			found = true
		}
	}
	if !found {
		t.Fatal("admin log delete route is not registered")
	}
}

// TestModuleRegistrarCoversFrontendActions 锁定模块商店的前端契约：页面会调用
// state/install/uninstall/changeState/dependentInstallComplete/upload，少注册
// 任何一个都会变成 404。handler 是桩实现，但路由必须在。
func TestModuleRegistrarCoversFrontendActions(t *testing.T) {
	engine := gin.New()
	NewModuleRegistrar(&admin.ModuleHandler{}).Register(engine.Group("/admin"))

	registered := make(map[string]bool)
	for _, route := range engine.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		"GET /admin/module/index",
		"GET /admin/module/state",
		"POST /admin/module/install",
		"POST /admin/module/uninstall",
		"POST /admin/module/changeState",
		"POST /admin/module/dependentInstallComplete",
		"POST /admin/module/update",
		"POST /admin/module/upload",
	} {
		require.True(t, registered[want], "module route %s is not registered", want)
	}
}

func TestAdminRouterRegistersPermissionExemptions(t *testing.T) {
	engine := gin.New()
	NewAdminRouter(AdminRouterDeps{
		LoginM:         &adminMiddleware.Login{},
		AuthorizationM: &adminMiddleware.Authorization{},
		SecurityM:      &adminMiddleware.Security{},
		RecordM:        &adminMiddleware.Record{},
		IndexHandler:   &admin.IndexHandler{},
		AjaxHandler:    &admin.AjaxHandler{},
	}).Register(engine)

	for _, exemption := range []struct {
		controller string
		action     string
	}{
		{"index", "index"},
		{"ajax", "area"},
		{"ajax", "upload"},
		{"alioss", "callback"},
	} {
		require.True(t, adminMiddleware.IsPermissionExempt(exemption.controller, exemption.action),
			"%s/%s should be permission exempt", exemption.controller, exemption.action)
	}

	for _, noLogin := range []struct {
		controller string
		action     string
	}{
		{"index", "login"},
		{"index", "logout"},
		{"ajax", "buildSuffixSvg"},
		{"ajax", "terminal"},
	} {
		require.True(t, adminMiddleware.IsNoNeedLogin(noLogin.controller, noLogin.action),
			"%s/%s should be login exempt", noLogin.controller, noLogin.action)
	}
}

func TestAdminRouterProtectedRouteRequiresLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// The Login middleware relies on the global i18n context for its abort
	// message, mirroring the composer's global chain.
	engine.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		DefaultLanguage: language.English,
	})))
	NewAdminRouter(AdminRouterDeps{
		LoginM:         &adminMiddleware.Login{},
		AuthorizationM: &adminMiddleware.Authorization{},
		SecurityM:      &adminMiddleware.Security{},
		RecordM:        &adminMiddleware.Record{},
		IndexHandler:   &admin.IndexHandler{},
		AjaxHandler:    &admin.AjaxHandler{},
	}).Register(engine)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/Index/index", nil)
	engine.ServeHTTP(recorder, request)

	// The protected group must run Login before the handler; without a token
	// Login aborts with the business 401 code.
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusUnauthorized, response.Code)
}

type fakeRegistrar struct {
	register func(r gin.IRoutes)
}

func (f fakeRegistrar) Register(r gin.IRoutes) { f.register(r) }

func (f fakeRegistrar) Capabilities() []middleware.AtomicRoute {
	return []middleware.AtomicRoute{
		{Route: "fake", Action: "add", Method: http.MethodPost},
	}
}

func TestAdminRouterRegistersRegistrarCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewAdminRouter(AdminRouterDeps{
		LoginM:         &adminMiddleware.Login{},
		AuthorizationM: &adminMiddleware.Authorization{},
		SecurityM:      &adminMiddleware.Security{},
		RecordM:        &adminMiddleware.Record{},
		IndexHandler:   &admin.IndexHandler{},
		AjaxHandler:    &admin.AjaxHandler{},
		Registrars: []Registrar{
			fakeRegistrar{register: func(r gin.IRoutes) {
				r.POST("fake/add", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			}},
		},
	}).Register(engine)

	var capability middleware.AtomicRoute
	var ok bool
	probe := gin.New()
	probe.Handle(http.MethodPost, "/admin/fake/add", func(c *gin.Context) {
		capability, ok = middleware.AtomicRouteCapability(c)
	})
	recorder := httptest.NewRecorder()
	probe.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/admin/fake/add", nil))

	require.True(t, ok)
	require.Equal(t, middleware.AtomicRoute{Route: "fake", Action: "add", Method: http.MethodPost}, capability)
}
