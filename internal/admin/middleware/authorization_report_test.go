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
		{Method: "GET", Path: "/admin/odd/shape/extra"},
		{Method: "GET", Path: "/api/outside/index"},
	}
	rules := map[string]struct{}{"known/route/index": {}}

	missing, unparseable := collectUnprotectedRoutes(routes, rules)
	want := []string{"POST /admin/missing.Route/run"}
	if len(missing) != len(want) || missing[0] != want[0] {
		t.Fatalf("missing routes = %#v, want %#v", missing, want)
	}
	// Non-three-segment /admin routes are surfaced explicitly instead of
	// being silently skipped; non-admin paths stay out of scope.
	wantUnparseable := []string{"GET /admin/odd/shape/extra"}
	if len(unparseable) != len(wantUnparseable) || unparseable[0] != wantUnparseable[0] {
		t.Fatalf("unparseable routes = %#v, want %#v", unparseable, wantUnparseable)
	}
}

func TestCollectUnprotectedRoutesReportsIndexIndexWhenNotExempt(t *testing.T) {
	wasExempt := IsPermissionExempt("index", "index")
	UnregisterPermissionExempt("index", "index")
	t.Cleanup(func() {
		if wasExempt {
			RegisterPermissionExempt("index", "index")
		}
	})

	routes := gin.RoutesInfo{{Method: "GET", Path: "/admin/Index/index"}}
	missing, unparseable := collectUnprotectedRoutes(routes, map[string]struct{}{})
	if len(missing) != 1 || missing[0] != "GET /admin/Index/index" {
		t.Fatalf("missing routes = %#v, want index/index", missing)
	}
	if len(unparseable) != 0 {
		t.Fatalf("unparseable routes = %#v, want none", unparseable)
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
