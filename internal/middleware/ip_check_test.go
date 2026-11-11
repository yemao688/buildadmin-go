package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"buildadmin-go/internal/pkg/util"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// newIpCheckTestEngine 装配 i18n 中间件（util.Lang 依赖 context 里的 i18n）、
// IpCheck 与写 204 的 handler；handlerCalled 用于确认命中时 handler 未执行。
func newIpCheckTestEngine(source NoAccessIPSource, handlerCalled *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         util.RootPath() + "/internal/i18n/locales",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
	})))
	engine.Use(IpCheck(source))
	engine.GET("/admin/test/index", func(c *gin.Context) {
		if handlerCalled != nil {
			*handlerCalled = true
		}
		c.Status(http.StatusNoContent)
	})
	return engine
}

func serveIpCheck(engine *gin.Engine, remoteAddr string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/test/index", nil)
	if remoteAddr != "" {
		request.RemoteAddr = remoteAddr
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func TestIpCheckAllowsEmptyOrFailedConfig(t *testing.T) {
	t.Run("空配置放行", func(t *testing.T) {
		called := false
		engine := newIpCheckTestEngine(func(*gin.Context) (string, error) {
			return "   \n\t ", nil
		}, &called)

		recorder := serveIpCheck(engine, "")

		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.True(t, called)
	})

	t.Run("null source 放行", func(t *testing.T) {
		called := false
		engine := newIpCheckTestEngine(nil, &called)

		recorder := serveIpCheck(engine, "")

		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.True(t, called)
	})

	t.Run("读取配置出错放行", func(t *testing.T) {
		called := false
		engine := newIpCheckTestEngine(func(*gin.Context) (string, error) {
			return "", errors.New("db down")
		}, &called)

		recorder := serveIpCheck(engine, "")

		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.True(t, called)
	})
}

func TestIpCheckBlocksBlacklistedIP(t *testing.T) {
	called := false
	engine := newIpCheckTestEngine(func(*gin.Context) (string, error) {
		return "192.0.2.1", nil
	}, &called)

	recorder := serveIpCheck(engine, "192.0.2.1:1234")

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.False(t, called, "命中黑名单时 handler 不应执行")

	var response struct {
		Code int    `json:"code"`
		Data any    `json:"data"`
		Msg  string `json:"msg"`
		Time int    `json:"time"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusForbidden, response.Code)
	require.Nil(t, response.Data)
	require.NotEmpty(t, response.Msg)
	require.Zero(t, response.Time)
}

func TestIpCheckMatchingRules(t *testing.T) {
	cases := []struct {
		name       string
		rules      string
		remoteAddr string
		blocked    bool
	}{
		{name: "精确 IP 命中", rules: "192.0.2.1", remoteAddr: "192.0.2.1:8080", blocked: true},
		{name: "精确 IP 未命中", rules: "192.0.2.9", remoteAddr: "192.0.2.1:8080", blocked: false},
		{name: "CIDR 命中", rules: "10.0.0.0/8", remoteAddr: "10.1.2.3:8080", blocked: true},
		{name: "CIDR 未命中", rules: "10.0.0.0/8", remoteAddr: "11.1.2.3:8080", blocked: false},
		{name: "通配命中", rules: "192.168.1.*", remoteAddr: "192.168.1.42:8080", blocked: true},
		{name: "通配未命中", rules: "192.168.1.*", remoteAddr: "192.168.2.42:8080", blocked: false},
		{name: "全通配命中", rules: "*", remoteAddr: "203.0.113.7:8080", blocked: true},
		{
			name:       "换行逗号CRLF空格混合分隔",
			rules:      "198.51.100.1,\r\n203.0.113.9  192.0.2.1\t10.0.0.0/8",
			remoteAddr: "10.9.8.7:8080",
			blocked:    true,
		},
		{
			name:       "非法规则不影响其它规则命中",
			rules:      "not-an-ip,10.0.0.0/99,192.0.2.1",
			remoteAddr: "192.0.2.1:8080",
			blocked:    true,
		},
		{
			name:       "非法规则全部跳过时放行",
			rules:      "not-an-ip,10.0.0.0/99,192.168.1.*",
			remoteAddr: "192.0.2.1:8080",
			blocked:    false,
		},
		{
			name:       "IPv4-mapped IPv6 客户端按 Unmap 比较",
			rules:      "192.0.2.1",
			remoteAddr: "[::ffff:192.0.2.1]:8080",
			blocked:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			engine := newIpCheckTestEngine(func(*gin.Context) (string, error) {
				return tc.rules, nil
			}, &called)

			recorder := serveIpCheck(engine, tc.remoteAddr)

			if tc.blocked {
				require.Equal(t, http.StatusForbidden, recorder.Code)
				require.False(t, called)
				return
			}
			require.Equal(t, http.StatusNoContent, recorder.Code)
			require.True(t, called)
		})
	}
}
