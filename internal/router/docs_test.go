package router

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buildadmin-go/internal/conf"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// newDocsTestEngine 构造带 public/docs/guide.html 的最小引擎，按参数决定
// 是否挂载 /docs 及其访问密码（绕过全局中间件与渠道注册器，聚焦挂载逻辑）。
// 返回引擎与 docsDir，供测试追加/断言文件。
func newDocsTestEngine(t *testing.T, enabled bool, password string) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	rootDir := t.TempDir()
	docsDir := filepath.Join(rootDir, "public", "docs")
	require.NoError(t, os.MkdirAll(docsDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "guide.html"), []byte("<h1>guide</h1>"), 0o644))

	engine := gin.New()
	config := &conf.Configuration{App: conf.App{DocsEnabled: enabled, DocsPassword: password}}
	if config.App.DocsEnabled {
		registerDocsRoute(engine, rootDir, config.App.DocsPassword)
	}
	return engine, docsDir
}

func serve(t *testing.T, engine *gin.Engine, method, path string, withCookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	if withCookie != nil {
		request.AddCookie(withCookie)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	return recorder
}

// TestDocsRouteDisabled 验证默认关闭：docs_enabled=false 时不挂载 /docs，
// 请求 404，行为与历史版本一致（升级零冲突）。
func TestDocsRouteDisabled(t *testing.T) {
	engine, _ := newDocsTestEngine(t, false, "")
	recorder := serve(t, engine, http.MethodGet, "/docs/guide.html", nil)
	require.Equal(t, http.StatusNotFound, recorder.Code)
}

// TestDocsRoutePublic 验证公开模式：密码为空时 /docs 等价于其它静态挂载，
// 直接出文件，/docs 精确路径 301 到 /docs/，目录请求（无 index.html）404
// 且不展示目录列表（与 router.Static 行为一致）。
func TestDocsRoutePublic(t *testing.T) {
	engine, docsDir := newDocsTestEngine(t, true, "")

	recorder := serve(t, engine, http.MethodGet, "/docs/guide.html", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "<h1>guide</h1>", recorder.Body.String())

	recorder = serve(t, engine, http.MethodGet, "/docs", nil)
	require.Equal(t, http.StatusMovedPermanently, recorder.Code)
	require.Equal(t, "/docs/", recorder.Header().Get("Location"))

	// 目录请求无 index.html：404，不泄漏文件列表
	recorder = serve(t, engine, http.MethodGet, "/docs/", nil)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "guide.html")

	// 目录含 index.html：正常渲染入口页（预写 404 被 FileServer 覆盖）
	require.NoError(t, os.WriteFile(filepath.Join(docsDir, "index.html"), []byte("<h1>docs index</h1>"), 0o644))
	recorder = serve(t, engine, http.MethodGet, "/docs/", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "<h1>docs index</h1>", recorder.Body.String())
}

// TestDocsRouteGated 验证密码门完整流程：未带 cookie 返回密码页（不回源
// 文件），密码提交成功 Set-Cookie 并回跳，带 cookie 后正常出文件，错误密码
// 重渲页面提示，HEAD 探测返回 401 空体。
func TestDocsRouteGated(t *testing.T) {
	const password = "s3cret"
	engine, _ := newDocsTestEngine(t, true, password)

	t.Run("未带 cookie 返回密码页且不泄漏文件", func(t *testing.T) {
		recorder := serve(t, engine, http.MethodGet, "/docs/guide.html", nil)
		require.Equal(t, http.StatusOK, recorder.Code)
		body := recorder.Body.String()
		require.Contains(t, body, `action="/docs/password"`)
		require.Contains(t, body, `name="next" value="/docs/guide.html"`)
		require.NotContains(t, body, "<h1>guide</h1>")
	})

	t.Run("错误密码重渲页面并提示", func(t *testing.T) {
		form := url.Values{"password": {"wrong"}, "next": {"/docs/guide.html"}}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/docs/password", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		engine.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Contains(t, recorder.Body.String(), "密码错误，请重试。")
	})

	t.Run("正确密码 Set-Cookie 并回跳", func(t *testing.T) {
		form := url.Values{"password": {password}, "next": {"/docs/guide.html"}}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/docs/password", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		engine.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusFound, recorder.Code)
		require.Equal(t, "/docs/guide.html", recorder.Header().Get("Location"))

		var authCookie *http.Cookie
		for _, cookie := range recorder.Result().Cookies() {
			if cookie.Name == docsCookieName {
				authCookie = cookie
			}
		}
		require.NotNil(t, authCookie, "应写入 %s cookie", docsCookieName)
		require.Equal(t, docsCookieValue(password), authCookie.Value)
		require.True(t, authCookie.HttpOnly)
		require.Equal(t, "/docs", authCookie.Path)
		require.Equal(t, docsCookieMaxAge, authCookie.MaxAge)
	})

	t.Run("带有效 cookie 正常出文件", func(t *testing.T) {
		cookie := &http.Cookie{Name: docsCookieName, Value: docsCookieValue(password)}
		recorder := serve(t, engine, http.MethodGet, "/docs/guide.html", cookie)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "<h1>guide</h1>", recorder.Body.String())
	})

	t.Run("带有效 cookie 目录请求 404 不展示列表", func(t *testing.T) {
		cookie := &http.Cookie{Name: docsCookieName, Value: docsCookieValue(password)}
		recorder := serve(t, engine, http.MethodGet, "/docs/", cookie)
		require.Equal(t, http.StatusNotFound, recorder.Code)
		require.NotContains(t, recorder.Body.String(), "guide.html")
	})

	t.Run("HEAD 未放行返回 401 空体", func(t *testing.T) {
		recorder := serve(t, engine, http.MethodHead, "/docs/guide.html", nil)
		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		require.Empty(t, recorder.Body.String())
	})
}

// TestDocsNextSanitization 验证 next 回跳只接受站内相对路径：外部 URL 与
// 协议相对 URL 一律回退到 /docs/，防密码页被当作开放重定向器。
func TestDocsNextSanitization(t *testing.T) {
	engine, _ := newDocsTestEngine(t, true, "s3cret")

	for _, malicious := range []string{"https://evil.example/x", "//evil.example/x", "javascript:alert(1)"} {
		form := url.Values{"password": {"s3cret"}, "next": {malicious}}
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/docs/password", strings.NewReader(form.Encode()))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		engine.ServeHTTP(recorder, request)

		require.Equal(t, http.StatusFound, recorder.Code, "next=%q", malicious)
		require.Equal(t, "/docs/", recorder.Header().Get("Location"), "next=%q", malicious)
	}
}

// TestDocsPasswordEnvOverride 验证环境变量 DOCS_PASSWORD 覆盖 YAML 密码：
// YAML 留空 + 环境变量非空时仍开启密码门，且用环境值可验密通过。
func TestDocsPasswordEnvOverride(t *testing.T) {
	t.Setenv("DOCS_PASSWORD", "env-s3cret")
	engine, _ := newDocsTestEngine(t, true, "")

	// 环境变量生效 → 未带 cookie 返回密码页而非源文件
	recorder := serve(t, engine, http.MethodGet, "/docs/guide.html", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `action="/docs/password"`)

	// 环境值可验密通过
	form := url.Values{"password": {"env-s3cret"}, "next": {"/docs/guide.html"}}
	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/docs/password", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusFound, recorder.Code)
}
