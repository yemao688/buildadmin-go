package middleware

import (
	adminauth "buildadmin-go/internal/admin/repository"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

func TestCollectUnprotectedRoutesReportsIndexIndexWhenNotExempt(t *testing.T) {
	UnregisterPermissionExempt("index", "index")

	routes := gin.RoutesInfo{{Method: "GET", Path: "/admin/Index/index"}}
	got := collectUnprotectedRoutes(routes, map[string]struct{}{})
	if len(got) != 1 || got[0] != "GET /admin/Index/index" {
		t.Fatalf("missing routes = %#v, want index/index", got)
	}
}

func TestReportUnprotectedRoutesSkipsNilDatabase(t *testing.T) {
	authorization := NewAuthorization(adminauth.NewAuthRepository(nil, nil, nil), zap.NewNop())

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("ReportUnprotectedRoutes panicked with nil database: %v", recovered)
		}
	}()
	authorization.ReportUnprotectedRoutes(gin.RoutesInfo{{Method: "GET", Path: "/admin/missing.Route/run"}})
}
