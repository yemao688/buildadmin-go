package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/util"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// newResponseShellEngine 装配 i18n 中间件（util.Lang 依赖 context 里的 i18n
// 实例）与单个探活 handler，用于锁定中间件响应壳的 JSON 形状。
func newResponseShellEngine(handler gin.HandlerFunc) *gin.Engine {
	engine := gin.New()
	engine.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         util.RootPath() + "/internal/i18n/locales",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
	})))
	engine.GET("/", handler)
	return engine
}

func serveResponseShell(engine *gin.Engine) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	return recorder
}

func decodeResponseShell(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), "body=%s", recorder.Body.String())
	return body
}

func TestAbortLoginWritesLoginShell(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
	var aborted bool
	engine := newResponseShellEngine(func(c *gin.Context) {
		AbortLogin(c, cErr.Unauthorized("Please login first"))
		aborted = c.IsAborted()
	})

	recorder := serveResponseShell(engine)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, aborted)
	require.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	body := decodeResponseShell(t, recorder)
	require.InDelta(t, float64(http.StatusUnauthorized), body["code"], 1e-9)
	require.Equal(t, "Please login first", body["msg"])
	require.Equal(t, map[string]any{"type": "need login"}, body["data"])
	require.Zero(t, body["time"])
}

func TestAbortLoginDefaultsToUnauthorizedForPlainError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
	engine := newResponseShellEngine(func(c *gin.Context) {
		AbortLogin(c, errors.New("token expired"))
	})

	body := decodeResponseShell(t, serveResponseShell(engine))

	require.InDelta(t, float64(http.StatusUnauthorized), body["code"], 1e-9)
	require.Equal(t, "Please login first", body["msg"])
}

func TestAbortMissingTokenWritesLoginShell(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
	var aborted bool
	engine := newResponseShellEngine(func(c *gin.Context) {
		AbortMissingToken(c)
		aborted = c.IsAborted()
	})

	recorder := serveResponseShell(engine)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, aborted)
	body := decodeResponseShell(t, recorder)
	require.InDelta(t, float64(http.StatusUnauthorized), body["code"], 1e-9)
	require.Nil(t, body["data"])
	require.NotEmpty(t, body["msg"])
	require.Zero(t, body["time"])
}

func TestServerErrorWritesInternalErrorShell(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Parallel()
	var aborted bool
	engine := newResponseShellEngine(func(c *gin.Context) {
		ServerError(c, errors.New("boom"))
		aborted = c.IsAborted()
	})

	recorder := serveResponseShell(engine)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.True(t, aborted)
	body := decodeResponseShell(t, recorder)
	require.InDelta(t, float64(cErr.ServerError), body["code"], 1e-9)
	require.Equal(t, "boom", body["msg"])
	require.Nil(t, body["data"])
	require.Zero(t, body["time"])
}
