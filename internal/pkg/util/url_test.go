package util

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetBaseURL(t *testing.T) {
	newCtx := func(tlsState bool, headers map[string]string) *gin.Context {
		gin.SetMode(gin.TestMode)
		req := httptest.NewRequest(http.MethodGet, "http://example.com/path", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		if tlsState {
			req.TLS = &tls.ConnectionState{}
		}
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = req
		return ctx
	}

	// Cloudflare/nginx TLS 终止：后端直连是 http，但转发头标 https
	require.Equal(t, "https://example.com", GetBaseURL(newCtx(false, map[string]string{"X-Forwarded-Proto": "https"})))
	// 多级代理逗号分隔取第一跳
	require.Equal(t, "https://example.com", GetBaseURL(newCtx(false, map[string]string{"X-Forwarded-Proto": "https, http"})))
	// X-Forwarded-Ssl 兼容
	require.Equal(t, "https://example.com", GetBaseURL(newCtx(false, map[string]string{"X-Forwarded-Ssl": "on"})))
	// 直连 TLS
	require.Equal(t, "https://example.com", GetBaseURL(newCtx(true, nil)))
	// 直连无 TLS 且无代理头 → http
	require.Equal(t, "http://example.com", GetBaseURL(newCtx(false, nil)))
	// 转发头优先于直连 TLS 判断
	require.Equal(t, "http://example.com", GetBaseURL(newCtx(true, map[string]string{"X-Forwarded-Proto": "http"})))
}
