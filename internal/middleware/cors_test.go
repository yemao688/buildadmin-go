package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCorsAllowHeadersWildcard(t *testing.T) {
	// 对齐 PHP 上游 AllowCrossDomain：Allow-Methods/Allow-Headers 为通配符，
	// 业务扩展 token 域（如 ba-seller-token）无需改动中间件即可通过预检。
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Cors("*"))
	router.GET("/api/index/index", func(c *gin.Context) { c.Status(http.StatusOK) })

	// 模拟业务门户跨域预检：扩展头 ba-seller-token 不在任何显式列表里。
	req := httptest.NewRequest(http.MethodOptions, "/api/index/index", nil)
	req.Header.Set("Origin", "http://portal.example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "think-lang, server, ba-seller-token")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	require.Equal(t, "*", w.Header().Get("Access-Control-Allow-Headers"))
	require.Equal(t, "*", w.Header().Get("Access-Control-Allow-Methods"))
	require.Equal(t, "http://portal.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestParseCorsDomains(t *testing.T) {
	require.Equal(t, []string{"localhost", "127.0.0.1"}, parseCorsDomains("localhost,127.0.0.1"))
	// 逐项 trim 空格并剔除空项（对齐 PHP 上游 explode + 配置中可能存在的
	// 空格分隔习惯）。
	require.Equal(t, []string{"example.com", "a.com"}, parseCorsDomains(" example.com , , a.com "))
	require.Empty(t, parseCorsDomains(""))
	require.Empty(t, parseCorsDomains(" , , "))
}

func TestOriginAllowed(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		selfHost string
		domains  []string
		want     bool
	}{
		{
			name:     "白名单含*放行",
			origin:   "https://evil.example.com",
			selfHost: "api.buildadmin.com",
			domains:  []string{"*"},
			want:     true,
		},
		{
			name:     "origin完整匹配白名单",
			origin:   "https://example.com",
			selfHost: "api.buildadmin.com",
			domains:  []string{"https://example.com"},
			want:     true,
		},
		{
			name:     "origin带端口host匹配无端口白名单项",
			origin:   "https://example.com:8443",
			selfHost: "api.buildadmin.com",
			domains:  []string{"example.com"},
			want:     true,
		},
		{
			name:     "origin不带端口host匹配白名单项",
			origin:   "http://example.com",
			selfHost: "api.buildadmin.com",
			domains:  []string{"example.com"},
			want:     true,
		},
		{
			name:     "白名单项带端口时按host比较不匹配",
			origin:   "https://example.com",
			selfHost: "api.buildadmin.com",
			domains:  []string{"example.com:8080"},
			want:     false,
		},
		{
			name:     "自身host放行（请求Host与origin host一致，忽略端口）",
			origin:   "http://localhost:9900",
			selfHost: "localhost",
			domains:  nil,
			want:     true,
		},
		{
			name:     "自身host放行（两边均带端口）",
			origin:   "http://localhost:9900",
			selfHost: "localhost:9900",
			domains:  []string{"127.0.0.1"},
			want:     true,
		},
		{
			name:     "不匹配拒绝",
			origin:   "https://evil.com",
			selfHost: "api.buildadmin.com",
			domains:  []string{"example.com", "127.0.0.1"},
			want:     false,
		},
		{
			name:     "白名单为空且非同源拒绝",
			origin:   "https://evil.com",
			selfHost: "api.buildadmin.com",
			domains:  nil,
			want:     false,
		},
		{
			// trim 发生在 parseCorsDomains（启动时解析），这里走真实解析
			// 管线验证：带空白的原始配置串解析后仍能命中。
			name:     "白名单项trim空格后仍命中",
			origin:   "https://example.com",
			selfHost: "api.buildadmin.com",
			domains:  parseCorsDomains(" example.com , "),
			want:     true,
		},
		{
			name:     "非法origin拒绝",
			origin:   "://bad",
			selfHost: "api.buildadmin.com",
			domains:  []string{"api.buildadmin.com"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, originAllowed(tt.origin, tt.selfHost, tt.domains))
		})
	}
}
