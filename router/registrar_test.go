package router

import (
	"net/http"
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

func newCompleteRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	admin.RegisteredRoutes = nil

	return InitRouter(
		&lumberjack.Logger{},
		&middleware.Login{},
		&middleware.Security{},
		&middleware.UserLogin{},
		&middleware.Record{},
		&admin.AdminHandler{},
		&admin.AdminInfoHandler{},
		&admin.AdminGroupHandler{},
		&admin.AdminRuleHandler{},
		&admin.AdminLogHandler{},
		&admin.TestBuildHandler{},
		&admin.IndexHandler{},
		&admin.DashboardHandler{},
		&admin.UserHandler{},
		&admin.UserGroupHandler{},
		&admin.UserRuleHandler{},
		&admin.UserMoneyLogHandler{},
		&admin.UserScoreLogHandler{},
		&admin.AttachmentHandler{},
		&admin.CrudHandler{},
		&admin.CrudLogHandler{},
		&admin.ConfigHandler{},
		&admin.ModuleHandler{},
		&admin.DataRecycleHandler{},
		&admin.DataRecycleLogHandler{},
		&admin.SensitiveDataHandler{},
		&admin.SensitiveDataLogHandler{},
		&admin.AjaxHandler{},
		&api.AccountHandler{},
		&api.AjaxHandler{},
		&api.CommonHandler{},
		&api.EmsHandler{},
		&api.IndexHandler{},
		&api.InstallHandler{},
		&api.UserHandler{},
		&api.DemoHandler{},
		ProvideRegistrars(
			admin.NewCountryLanguageRegistrar(&admin.CountryLanguageHandler{}),
			admin.NewCountryCurrencyRegistrar(&admin.CountryCurrencyHandler{}),
			admin.NewCountryLanguageContentRegistrar(&admin.CountryLanguageContentHandler{}),
		),
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
