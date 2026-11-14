package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"buildadmin-go/internal/conf"
	middlewarecore "buildadmin-go/internal/middleware"
	"buildadmin-go/internal/pkg/data_scope"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTransactionResponseWriterIsolatesHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
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

//nolint:paralleltest // 注册/注销全局原子路由能力表，并行会被其它测试的同名注销提前移除
func TestAtomicRouteCapabilityNormalizesRegisteredRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// The capability map is populated by route registration; an isolated
	// construction must declare the capability explicitly.
	middlewarecore.RegisterAtomicRoute(middlewarecore.AtomicRoute{Route: "auth/admin", Action: "edit", Method: http.MethodPost})
	t.Cleanup(func() {
		middlewarecore.UnregisterAtomicRoute(middlewarecore.AtomicRoute{Route: "auth/admin", Action: "edit", Method: http.MethodPost})
	})

	r := gin.New()
	r.POST("/admin/auth.Admin/edit", func(_ *gin.Context) {})
	req := httptest.NewRequest(http.MethodPost, "/admin/auth.Admin/edit", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	r.HandleContext(c)

	capability, ok := middlewarecore.AtomicRouteCapability(c)
	if !ok || capability.Route != "auth/admin" || capability.Action != "edit" {
		t.Fatalf("unexpected atomic capability: %#v, %v", capability, ok)
	}
}

func TestAtomicRouteCapabilityRejectsUnregisteredRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
	r := gin.New()
	r.POST("/admin/unknown.Action/run", func(_ *gin.Context) {})
	req := httptest.NewRequest(http.MethodPost, "/admin/unknown.Action/run", nil)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	r.HandleContext(c)
	if _, ok := middlewarecore.AtomicRouteCapability(c); ok {
		t.Fatal("unregistered route must not have atomic capability")
	}
}

func TestSecurityMissingActorAborts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
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
	t.Parallel()
	security := NewSecurity(nil, zap.NewNop(), nil, data_scope.NewDenyAllEnforcer())
	_, err := security.hasSecurityRule(nil, "auth/admin")
	if err == nil {
		t.Fatal("security rule database errors must not be treated as no rule")
	}
}

func TestNormalizeAuditValueStableDriverValues(t *testing.T) {
	t.Parallel()
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

// newTransactionShellRouter 装配走事务分支的 Security 中间件：注册 auth/admin
// edit 的原子路由能力，security_sensitive_data 存在但无匹配规则，因此审计器
// 直接放行到业务 handler，由 punch 决定暂存哪种结果。
func newTransactionShellRouter(t *testing.T, punch gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:security-transaction-shell?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS security_sensitive_data (id INTEGER PRIMARY KEY, status TEXT, controller_as TEXT)").Error; err != nil {
		t.Fatalf("create rule table: %v", err)
	}
	middlewarecore.RegisterAtomicRoute(middlewarecore.AtomicRoute{Route: "auth/admin", Action: "edit", Method: http.MethodPost})
	t.Cleanup(func() {
		middlewarecore.UnregisterAtomicRoute(middlewarecore.AtomicRoute{Route: "auth/admin", Action: "edit", Method: http.MethodPost})
	})

	security := NewSecurity(&conf.Configuration{}, zap.NewNop(), db, data_scope.NewDenyAllEnforcer())
	router := gin.New()
	router.Use(func(c *gin.Context) {
		_ = data_scope.SetActor(c, data_scope.Actor{AdminID: 1, Unrestricted: true})
	}, security.Handler())
	router.POST("/admin/auth.Admin/edit", punch)
	return router
}

func serveTransactionShell(t *testing.T, router *gin.Engine) (int, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/auth.Admin/edit", bytes.NewBufferString(`{"id":1}`))
	router.ServeHTTP(recorder, request)
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %q", recorder.Body.String())
	}
	return recorder.Code, body
}

// 事务回滚、提交与失败三条收尾路径都必须输出统一响应壳（time 固定 0）。
//
//nolint:paralleltest // 注册全局原子路由并复用同名 sqlite 内存库，子测试并行会互相注销能力/串库
func TestSecurityTransactionEmitsUnifiedShell(t *testing.T) {
	t.Run("业务失败回滚输出已暂存响应", func(t *testing.T) { //nolint:paralleltest // newTransactionShellRouter 注册全局原子路由且复用同名 sqlite 内存库，并行会互相注销/串库
		router := newTransactionShellRouter(t, func(c *gin.Context) {
			stage(c, 0, "boom")
		})
		status, body := serveTransactionShell(t, router)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if body["code"] != float64(0) || body["msg"] != "boom" || body["data"] != nil || body["time"] != float64(0) {
			t.Fatalf("unexpected rollback shell: %v", body)
		}
	})

	t.Run("成功提交输出已暂存响应", func(t *testing.T) { //nolint:paralleltest // newTransactionShellRouter 注册全局原子路由且复用同名 sqlite 内存库，并行会互相注销/串库
		router := newTransactionShellRouter(t, func(c *gin.Context) {
			stage(c, 1, "")
		})
		status, body := serveTransactionShell(t, router)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want %d", status, http.StatusOK)
		}
		if body["code"] != float64(1) || body["msg"] != "" || body["time"] != float64(0) {
			t.Fatalf("unexpected commit shell: %v", body)
		}
	})

	t.Run("缺少暂存响应输出事务失败", func(t *testing.T) { //nolint:paralleltest // newTransactionShellRouter 注册全局原子路由且复用同名 sqlite 内存库，并行会互相注销/串库
		router := newTransactionShellRouter(t, func(_ *gin.Context) {})
		status, body := serveTransactionShell(t, router)
		if status != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", status, http.StatusInternalServerError)
		}
		if body["code"] != float64(0) || body["msg"] != "transaction failed" || body["data"] != nil || body["time"] != float64(0) {
			t.Fatalf("unexpected failure shell: %v", body)
		}
	})
}
