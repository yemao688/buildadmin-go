package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	admin "go-build-admin/internal/admin/handler"
	authhandler "go-build-admin/internal/admin/handler/auth"
	adminMiddleware "go-build-admin/internal/admin/middleware"
	"go-build-admin/internal/middleware"

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
	}).Register(engine, []Registrar{
		authhandler.NewAdminLogRegistrar(&authhandler.AdminLogHandler{}),
	})

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

func TestAdminRouterRegistersPermissionExemptions(t *testing.T) {
	engine := gin.New()
	NewAdminRouter(AdminRouterDeps{
		LoginM:         &adminMiddleware.Login{},
		AuthorizationM: &adminMiddleware.Authorization{},
		SecurityM:      &adminMiddleware.Security{},
		RecordM:        &adminMiddleware.Record{},
		IndexHandler:   &admin.IndexHandler{},
		AjaxHandler:    &admin.AjaxHandler{},
	}).Register(engine, nil)

	for _, exemption := range []struct {
		controller string
		action     string
	}{
		{"index", "index"},
		{"index", "logout"},
		{"ajax", "area"},
		{"ajax", "upload"},
		{"alioss", "callback"},
	} {
		require.True(t, adminMiddleware.IsPermissionExempt(exemption.controller, exemption.action),
			"%s/%s should be permission exempt", exemption.controller, exemption.action)
	}
}

func TestAdminRouterProtectedRouteRequiresLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	// The Login middleware relies on the global i18n context for its abort
	// message, mirroring the composer's global chain.
	engine.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		DefaultLanguage: language.Chinese,
	})))
	NewAdminRouter(AdminRouterDeps{
		LoginM:         &adminMiddleware.Login{},
		AuthorizationM: &adminMiddleware.Authorization{},
		SecurityM:      &adminMiddleware.Security{},
		RecordM:        &adminMiddleware.Record{},
		IndexHandler:   &admin.IndexHandler{},
		AjaxHandler:    &admin.AjaxHandler{},
	}).Register(engine, nil)

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
	}).Register(engine, []Registrar{
		fakeRegistrar{register: func(r gin.IRoutes) {
			r.POST("fake/add", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		}},
	})

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
