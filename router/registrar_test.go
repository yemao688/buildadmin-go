package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	admin "go-build-admin/app/admin/handler"
	api "go-build-admin/app/api/handler"
	"go-build-admin/app/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gopkg.in/natefinch/lumberjack.v2"
)

func TestInitRouterCountryRegistrarRoutesAreCollectedWithoutDuplicates(t *testing.T) {
	engine := newCompleteRouter()

	want := countryRoutes()
	engineRoutes := uniqueRoutes(t, engine.Routes())
	require.Len(t, engineRoutes, len(engine.Routes()))
	for _, route := range want {
		require.Contains(t, engineRoutes, route)
	}

	collectedRoutes := uniqueCollectedRoutes(t, admin.GetAllRoutes())
	require.Len(t, collectedRoutes, len(admin.GetAllRoutes()))
	for _, route := range want {
		require.Contains(t, collectedRoutes, route)
	}
}

func TestInitRouterReplacesCollectedRoutesOnReinitialization(t *testing.T) {
	newCompleteRouter()

	newEngine := newCompleteRouter()
	collectedRoutes := admin.GetAllRoutes()

	uniqueCollectedRoutes(t, collectedRoutes)
	require.Len(t, collectedRoutes, len(newEngine.Routes()))

	want := make([]admin.Route, 0, len(newEngine.Routes()))
	for _, route := range newEngine.Routes() {
		want = append(want, admin.Route{
			Method:  route.Method,
			Path:    route.Path,
			Handler: route.Handler,
		})
	}
	require.Equal(t, want, collectedRoutes)
}

func TestRegistrarCapabilitiesMatchRegisteredRoutes(t *testing.T) {
	engine := newCompleteRouter()

	want := make(map[middleware.AtomicRoute]struct{})
	capabilityCount := 0
	for _, registrar := range completeRegistrars() {
		for _, capability := range registrar.Capabilities() {
			capabilityCount++
			if _, exists := want[capability]; exists {
				t.Fatalf("duplicate registrar capability: %#v", capability)
			}
			want[capability] = struct{}{}
		}
	}
	if capabilityCount == 0 {
		t.Fatal("registrars must declare atomic capabilities")
	}

	got := make(map[middleware.AtomicRoute]struct{})
	for _, route := range engine.Routes() {
		if !strings.HasPrefix(route.Path, "/admin/") {
			continue
		}
		capability, ok := lookupAtomicRouteCapability(t, route)
		if ok {
			got[capability] = struct{}{}
		}
	}

	if len(got) != len(want) {
		t.Fatalf("registered capability count = %d, registrar capability count = %d\nregistered: %#v\nregistrars: %#v", len(got), len(want), got, want)
	}
	for capability := range want {
		if _, ok := got[capability]; !ok {
			t.Fatalf("registrar capability has no matching registered route: %#v", capability)
		}
	}
	for capability := range got {
		if _, ok := want[capability]; !ok {
			t.Fatalf("registered route has no matching registrar capability: %#v", capability)
		}
	}
}

func lookupAtomicRouteCapability(t *testing.T, route gin.RouteInfo) (middleware.AtomicRoute, bool) {
	t.Helper()

	var capability middleware.AtomicRoute
	var ok bool
	router := gin.New()
	router.Handle(route.Method, route.Path, func(c *gin.Context) {
		capability, ok = middleware.AtomicRouteCapability(c)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(route.Method, route.Path, nil)
	router.ServeHTTP(recorder, request)
	return capability, ok
}

func newCompleteRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	return InitRouter(
		&lumberjack.Logger{},
		&middleware.Login{},
		&middleware.Security{},
		&middleware.UserLogin{},
		&middleware.Record{},
		&admin.IndexHandler{},
		&admin.AjaxHandler{},
		&api.InstallHandler{},
		completeRegistrars(),
	)
}

func completeRegistrars() []RouteRegistrar {
	return ProvideRegistrars(
		admin.NewCountryLanguageRegistrar(&admin.CountryLanguageHandler{}),
		admin.NewCountryCurrencyRegistrar(&admin.CountryCurrencyHandler{}),
		admin.NewCountryLanguageContentRegistrar(&admin.CountryLanguageContentHandler{}),
		admin.NewCrudLogRegistrar(&admin.CrudLogHandler{}),
		admin.NewModuleRegistrar(&admin.ModuleHandler{}),
		admin.NewTestBuildRegistrar(&admin.TestBuildHandler{}),
		admin.NewAdminGroupRegistrar(&admin.AdminGroupHandler{}),
		admin.NewAdminRuleRegistrar(&admin.AdminRuleHandler{}),
		admin.NewUserGroupRegistrar(&admin.UserGroupHandler{}),
		admin.NewUserRuleRegistrar(&admin.UserRuleHandler{}),
		admin.NewConfigRegistrar(&admin.ConfigHandler{}),
		admin.NewAttachmentRegistrar(&admin.AttachmentHandler{}),
		admin.NewAdminRegistrar(&admin.AdminHandler{}),
		admin.NewUserRegistrar(&admin.UserHandler{}),
		admin.NewDataRecycleRegistrar(&admin.DataRecycleHandler{}),
		admin.NewDataRecycleLogRegistrar(&admin.DataRecycleLogHandler{}),
		admin.NewSensitiveDataRegistrar(&admin.SensitiveDataHandler{}),
		admin.NewSensitiveDataLogRegistrar(&admin.SensitiveDataLogHandler{}),
		admin.NewAdminInfoRegistrar(&admin.AdminInfoHandler{}),
		admin.NewAdminLogRegistrar(&admin.AdminLogHandler{}),
		admin.NewCrudRegistrar(&admin.CrudHandler{}),
		admin.NewDashboardRegistrar(&admin.DashboardHandler{}),
		admin.NewUserLogRegistrar(&admin.UserHandler{}, &admin.UserMoneyLogHandler{}, &admin.UserScoreLogHandler{}),
		api.NewAccountRegistrar(&api.AccountHandler{}),
		api.NewAjaxRegistrar(&api.AjaxHandler{}),
		api.NewCommonRegistrar(&api.CommonHandler{}),
		api.NewEmsRegistrar(&api.EmsHandler{}),
		api.NewIndexRegistrar(&api.IndexHandler{}),
		api.NewUserRegistrar(&api.UserHandler{}),
		api.NewDemoRegistrar(&api.DemoHandler{}),
	)
}

func countryRoutes() []string {
	routes := make([]string, 0, 18)
	for _, name := range []string{"countryLanguage", "countryCurrency", "countryLanguageContent"} {
		for _, route := range []struct {
			method string
			path   string
		}{
			{http.MethodGet, "/admin/" + name + "/index"},
			{http.MethodPost, "/admin/" + name + "/add"},
			{http.MethodGet, "/admin/" + name + "/edit"},
			{http.MethodPost, "/admin/" + name + "/edit"},
			{http.MethodDelete, "/admin/" + name + "/del"},
			{http.MethodPost, "/admin/" + name + "/sortable"},
		} {
			routes = append(routes, routeKey(route.method, route.path))
		}
	}
	return routes
}

func uniqueRoutes(t *testing.T, routes []gin.RouteInfo) map[string]struct{} {
	result := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		key := routeKey(route.Method, route.Path)
		if _, exists := result[key]; exists {
			t.Fatalf("duplicate route %s", key)
		}
		result[key] = struct{}{}
	}
	return result
}

func uniqueCollectedRoutes(t *testing.T, routes []admin.Route) map[string]struct{} {
	result := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		key := routeKey(route.Method, route.Path)
		if _, exists := result[key]; exists {
			t.Fatalf("duplicate collected route %s", key)
		}
		result[key] = struct{}{}
	}
	return result
}

func routeKey(method, path string) string {
	return method + " " + path
}
