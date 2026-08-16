package i18n

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"zh-cn 映射到 zh-CN pack key", "zh-cn", "zh-CN"},
		{"zh-CN 大写映射到 zh-CN", "zh-CN", "zh-CN"},
		{"zh 泛化标识映射到 zh-CN", "zh", "zh-CN"},
		{"zh-hant 映射到 zh-Hant", "zh-hant", "zh-Hant"},
		{"zh-Hant 原格式映射到 zh-Hant", "zh-Hant", "zh-Hant"},
		{"zh-tw 映射到 zh-Hant", "zh-tw", "zh-Hant"},
		{"zh-TW 大写映射到 zh-Hant", "zh-TW", "zh-Hant"},
		{"en 原样", "en", "en"},
		{"EN 大写归一为 en", "EN", "en"},
		{"未知语言原样返回", "ja", "ja"},
		{"空字符串原样返回", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, NormalizeLang(tt.in))
		})
	}
}

func TestLangContextHelpers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 未写入时读取返回 ok=false
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/index/index", nil)
	_, ok := LangFromContext(c)
	require.False(t, ok)

	// 写入后可从 *gin.Context 直读
	SetLangToContext(c, "zh-Hant")
	lang, ok := LangFromContext(c)
	require.True(t, ok)
	require.Equal(t, "zh-Hant", lang)

	// 派生 context（WithValue 链，父为 *gin.Context 或请求 context）可读到
	derived := context.WithValue(c, "k", "v")
	lang, ok = LangFromContext(derived)
	require.True(t, ok)
	require.Equal(t, "zh-Hant", lang)
	requestDerived := context.WithValue(c.Request.Context(), "k2", "v2")
	lang, ok = LangFromContext(requestDerived)
	require.True(t, ok)
	require.Equal(t, "zh-Hant", lang)

	// 普通 context 无语言
	_, ok = LangFromContext(context.Background())
	require.False(t, ok)
	_, ok = LangFromContext(nil)
	require.False(t, ok)

	// 空字符串视为未设置；nil Request 不应 panic
	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetLangToContext(c2, "")
	_, ok = LangFromContext(c2)
	require.False(t, ok)
	c3, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetLangToContext(c3, "en")
	lang, ok = LangFromContext(c3)
	require.True(t, ok)
	require.Equal(t, "en", lang)
	SetLangToContext(nil, "en")
}
