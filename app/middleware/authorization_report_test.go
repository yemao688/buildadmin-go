package middleware

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCollectUnprotectedRoutesFiltersRulesExemptionsAndBypasses(t *testing.T) {
	RegisterPermissionExempt("report.Exempt", "free")
	t.Cleanup(func() { UnregisterPermissionExempt("report.Exempt", "free") })

	routes := gin.RoutesInfo{
		{Method: "GET", Path: "/admin/known.Route/index"},
		{Method: "POST", Path: "/admin/report.Exempt/free"},
		{Method: "POST", Path: "/admin/missing.Route/run"},
		{Method: "GET", Path: "/admin/Index/login"},
		{Method: "GET", Path: "/admin/ajax/terminal"},
		{Method: "GET", Path: "/api/outside/index"},
	}
	rules := map[string]struct{}{"known/route/index": {}}

	got := collectUnprotectedRoutes(routes, rules)
	want := []string{"POST /admin/missing.Route/run"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("missing routes = %#v, want %#v", got, want)
	}
}
