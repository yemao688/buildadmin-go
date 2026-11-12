package util

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func clientIPContext(remoteAddr string, headers map[string]string) *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)
	ctx.Request.RemoteAddr = remoteAddr
	for key, value := range headers {
		ctx.Request.Header.Set(key, value)
	}
	return ctx
}

func TestGetClientIPIgnoresForwardedHeadersFromPublicPeer(t *testing.T) {
	ctx := clientIPContext("203.0.113.5:1234", map[string]string{
		"CF-Connecting-IP": "198.51.100.9",
		"X-Forwarded-For":  "198.51.100.8",
	})
	require.Equal(t, "203.0.113.5", GetClientIP(ctx))
}

func TestGetClientIPTrustsForwardedHeadersFromLoopback(t *testing.T) {
	ctx := clientIPContext("127.0.0.1:1234", map[string]string{
		"CF-Connecting-IP": "198.51.100.9",
	})
	require.Equal(t, "198.51.100.9", GetClientIP(ctx))
}

func TestGetClientIPTrustsForwardedHeadersFromPrivatePeer(t *testing.T) {
	ctx := clientIPContext("10.0.0.2:1234", map[string]string{
		"X-Forwarded-For": "198.51.100.7, 10.0.0.1",
	})
	require.Equal(t, "198.51.100.7", GetClientIP(ctx))
}

func TestGetClientIPHandlesMappedLoopback(t *testing.T) {
	ctx := clientIPContext("[::1]:1234", map[string]string{
		"CF-Connecting-IP": "198.51.100.11",
	})
	require.Equal(t, "198.51.100.11", GetClientIP(ctx))
}
