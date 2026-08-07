package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHealthRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerHealthRoute(router)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
}

func TestRootRouteServesIndex(t *testing.T) {
	const indexContent = "frontend index"

	rootDir := t.TempDir()
	publicDir := filepath.Join(rootDir, "public")
	require.NoError(t, os.MkdirAll(publicDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(publicDir, "index.html"), []byte(indexContent), 0o644))

	engine := gin.New()
	registerRootRoute(engine, rootDir)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, indexContent, recorder.Body.String())
}

// newLangTestContext 构造带指定请求路径与 think-lang header 的 gin.Context。
func newLangTestContext(t *testing.T, path, thinkLang string) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if thinkLang != "" {
		c.Request.Header.Set("think-lang", thinkLang)
	}
	return c
}

func TestResolveRequestLang(t *testing.T) {
	gin.SetMode(gin.TestMode)
	defaultLanFunc := func(lan string, err error) func(ctx context.Context) (string, error) {
		return func(context.Context) (string, error) { return lan, err }
	}

	t.Run("/api 无 header 取前台默认语言并规范化", func(t *testing.T) {
		c := newLangTestContext(t, "/api/index/index", "")
		// DefaultLan 返回 seed 值 zh-cn，经 NormalizeLang → zh（pack key）
		got := resolveRequestLang(c, "zh", defaultLanFunc("zh-cn", nil))
		require.Equal(t, "zh", got)
	})

	t.Run("/api 无 header 前台默认 en 时返回 en", func(t *testing.T) {
		c := newLangTestContext(t, "/api/index/index", "")
		got := resolveRequestLang(c, "zh", defaultLanFunc("en", nil))
		require.Equal(t, "en", got)
	})

	t.Run("/api 无 header 空表兜底 zh", func(t *testing.T) {
		c := newLangTestContext(t, "/api/index/index", "")
		got := resolveRequestLang(c, "zh", defaultLanFunc("", nil))
		require.Equal(t, "zh", got)
	})

	t.Run("/api 无 header 查询出错兜底 zh", func(t *testing.T) {
		c := newLangTestContext(t, "/api/index/index", "")
		got := resolveRequestLang(c, "zh", defaultLanFunc("", errors.New("db down")))
		require.Equal(t, "zh", got)
	})

	t.Run("/api 有 header 时 header 优先且规范化", func(t *testing.T) {
		c := newLangTestContext(t, "/api/index/index", "zh-cn")
		got := resolveRequestLang(c, "zh", defaultLanFunc("en", nil))
		require.Equal(t, "zh", got)
	})

	t.Run("/api 有 header zh-hant 映射到 zh-Hant", func(t *testing.T) {
		c := newLangTestContext(t, "/api/index/index", "zh-hant")
		got := resolveRequestLang(c, "zh", defaultLanFunc("zh-cn", nil))
		require.Equal(t, "zh-Hant", got)
	})

	t.Run("/admin 无 header 默认 zh", func(t *testing.T) {
		c := newLangTestContext(t, "/admin/index/index", "")
		got := resolveRequestLang(c, "zh", defaultLanFunc("en", nil))
		require.Equal(t, "zh", got)
	})

	t.Run("/admin 有 header en 时返回 en", func(t *testing.T) {
		c := newLangTestContext(t, "/admin/index/index", "en")
		got := resolveRequestLang(c, "zh", defaultLanFunc("zh-cn", nil))
		require.Equal(t, "en", got)
	})

	t.Run("未知路径无 header 默认 zh", func(t *testing.T) {
		c := newLangTestContext(t, "/", "")
		got := resolveRequestLang(c, "zh", defaultLanFunc("en", nil))
		require.Equal(t, "zh", got)
	})

	t.Run("未知语言原样返回", func(t *testing.T) {
		c := newLangTestContext(t, "/api/index/index", "ja")
		got := resolveRequestLang(c, "zh", defaultLanFunc("zh-cn", nil))
		require.Equal(t, "ja", got)
	})
}

// TestResolveRequestLangCachedPerRequest 验证 H1 每请求缓存：首次解析后结果
// 写入 gin context，同一请求内后续解析直接命中缓存，不再重复查库/解析。
func TestResolveRequestLangCachedPerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	queries := 0
	portalDefaultLang := func(context.Context) (string, error) {
		queries++
		return "en", nil
	}

	// /api 无 header：首次解析查一次默认语言，之后走 context 缓存
	c := newLangTestContext(t, "/api/index/index", "")
	require.Equal(t, "en", resolveRequestLang(c, "zh", portalDefaultLang))
	require.Equal(t, 1, queries)
	require.Equal(t, "en", resolveRequestLang(c, "zh", portalDefaultLang))
	require.Equal(t, "en", resolveRequestLang(c, "zh", portalDefaultLang))
	require.Equal(t, 1, queries)

	// 有 header 时同样只解析一次
	c = newLangTestContext(t, "/api/index/index", "zh-hant")
	require.Equal(t, "zh-Hant", resolveRequestLang(c, "zh", portalDefaultLang))
	require.Equal(t, "zh-Hant", resolveRequestLang(c, "zh", portalDefaultLang))
	require.Equal(t, 1, queries)

	// /admin 默认 zh 也只解析一次；不同请求（新 context）互不影响
	c = newLangTestContext(t, "/admin/index/index", "")
	require.Equal(t, "zh", resolveRequestLang(c, "zh", portalDefaultLang))
	require.Equal(t, "zh", resolveRequestLang(c, "zh", portalDefaultLang))
	require.Equal(t, 1, queries)
}
