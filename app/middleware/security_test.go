package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go-build-admin/app/pkg/data_scope"
	"go.uber.org/zap"
)

func TestTransactionResponseWriterIsolatesHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	w := &transactionResponseWriter{ResponseWriter: ctx.Writer, status: http.StatusCreated, header: make(http.Header)}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Add("Set-Cookie", "sid=commit")
	_, _ = w.WriteString("committed")
	w.flush()
	if got := rec.Code; got != http.StatusCreated {
		t.Fatalf("commit status = %d, want %d", got, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("commit content-type = %q", got)
	}
	if got := rec.Header().Get("Set-Cookie"); got != "sid=commit" {
		t.Fatalf("commit cookie = %q", got)
	}

	rollback := httptest.NewRecorder()
	rollbackCtx, _ := gin.CreateTestContext(rollback)
	failed := &transactionResponseWriter{ResponseWriter: rollbackCtx.Writer, status: http.StatusOK, header: make(http.Header)}
	failed.Header().Set("Content-Type", "text/plain")
	failed.Header().Set("Set-Cookie", "sid=rollback")
	_, _ = failed.WriteString("discarded")
	// A rollback path never calls flush, so staged headers cannot escape.
	rollback.Header().Set("Content-Type", "application/json")
	rollback.WriteHeader(http.StatusInternalServerError)
	_, _ = rollback.WriteString(`{"code":0,"msg":"transaction failed"}`)
	if got := rollback.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("rollback content-type = %q", got)
	}
	if got := rollback.Header().Get("Set-Cookie"); got != "" {
		t.Fatalf("rollback leaked cookie %q", got)
	}
}

func TestAtomicRouteCapabilityNormalizesRegisteredRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/admin/auth.Admin/edit", func(c *gin.Context) {})
	req := httptest.NewRequest(http.MethodPost, "/admin/auth.Admin/edit", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	r.HandleContext(c)

	cap, ok := AtomicRouteCapability(c)
	if !ok || cap.Route != "auth/admin" || cap.Action != "edit" {
		t.Fatalf("unexpected atomic capability: %#v, %v", cap, ok)
	}
}

func TestAtomicRouteCapabilityRejectsUnregisteredRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/admin/unknown.Action/run", func(c *gin.Context) {})
	req := httptest.NewRequest(http.MethodPost, "/admin/unknown.Action/run", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	r.HandleContext(c)
	if _, ok := AtomicRouteCapability(c); ok {
		t.Fatal("unregistered route must not have atomic capability")
	}
}

func TestSecurityMissingActorAborts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(r)
	c.Request = httptest.NewRequest("DELETE", "/admin/test/index", nil)
	security := NewSecurity(nil, zap.NewNop(), nil, data_scope.NewDenyAllEnforcer())
	h := security.Handler()
	h(c)
	if !c.IsAborted() {
		t.Fatal("missing actor must abort before downstream handling")
	}
}

func TestHasSecurityRuleReportsDatabaseUnavailable(t *testing.T) {
	security := NewSecurity(nil, zap.NewNop(), nil, data_scope.NewDenyAllEnforcer())
	_, err := security.hasSecurityRule(nil, "auth/admin")
	if err == nil {
		t.Fatal("security rule database errors must not be treated as no rule")
	}
}

func TestNormalizeAuditValueStableDriverValues(t *testing.T) {
	if got := normalizeAuditValue([]byte("hello")); got != "hello" {
		t.Fatalf("[]byte must normalize as text, got %q", got)
	}
	if got := normalizeAuditValue(float64(12.5)); got != "12.5" {
		t.Fatalf("unexpected numeric normalization: %q", got)
	}
	when := time.Date(2026, 7, 15, 4, 5, 6, 0, time.UTC)
	if got := normalizeAuditValue(when); got != "2026-07-15T04:05:06Z" {
		t.Fatalf("unexpected date normalization: %q", got)
	}
}
