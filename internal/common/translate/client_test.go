package translate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"buildadmin-go/internal/conf"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// serveBody 起一个返回固定 body 的翻译服务桩，并返回客户端与其 URL。
func serveBody(t *testing.T, status int, body string) *Client {
	t.Helper()
	// handler 跑在 httptest 的服务端 goroutine 里，不能用 require：require 的
	// t.FailNow 只终止当前 goroutine，测试仍会继续断言半成品响应。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/translateMulti", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewClient(&conf.Configuration{Translate: conf.Translate{API: srv.URL + "/"}})
}

func TestTranslateMultiSuccessMapping(t *testing.T) {
	t.Parallel()
	c := serveBody(t, http.StatusOK, `{"code":1,"data":[{"lan_to":"zh-cn","text_to":"你好"},{"lan_to":"en","text_to":"Hello"}]}`)

	got, err := c.TranslateMulti(context.Background(), "ja", []string{"zh-cn", "en"}, "こんにちは")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"zh-cn": "你好", "en": "Hello"}, got)
}

// 兼容部分部署网关把业务体再包一层 res 的形态。
func TestTranslateMultiWrappedResBody(t *testing.T) {
	t.Parallel()
	c := serveBody(t, http.StatusOK, `{"code":0,"res":{"code":1,"data":[{"lan_to":"en","text_to":"world"}]}}`)

	got, err := c.TranslateMulti(context.Background(), "ja", []string{"en"}, "世界")
	require.NoError(t, err)
	require.Equal(t, map[string]string{"en": "world"}, got)
}

func TestTranslateMultiHTTPStatusError(t *testing.T) {
	t.Parallel()
	c := serveBody(t, http.StatusInternalServerError, `{"code":0,"msg":"boom"}`)

	_, err := c.TranslateMulti(context.Background(), "ja", []string{"en"}, "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "http 500")
}

func TestTranslateMultiBusinessCodeError(t *testing.T) {
	t.Parallel()
	c := serveBody(t, http.StatusOK, `{"code":0,"msg":"unknow language"}`)

	_, err := c.TranslateMulti(context.Background(), "ja", []string{"en"}, "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknow language")
}

func TestTranslateMultiEmptyDataError(t *testing.T) {
	t.Parallel()
	c := serveBody(t, http.StatusOK, `{"code":1,"data":[]}`)

	_, err := c.TranslateMulti(context.Background(), "ja", []string{"en"}, "x")
	require.Error(t, err)
}

func TestTranslateMultiNoTargetLanguages(t *testing.T) {
	t.Parallel()
	c := serveBody(t, http.StatusOK, `{"code":1,"data":[]}`)

	_, err := c.TranslateMulti(context.Background(), "ja", nil, "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no target languages")
}

func TestTranslateMultiFallsBackOnMissingConfig(t *testing.T) {
	t.Parallel()
	// config 缺 translate 段时客户端仍可用（走代码兜底），且能正常请求。
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":1,"data":[{"lan_to":"en","text_to":"ok"}]}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(nil)
	require.NotNil(t, c)
	require.Equal(t, defaultAPI, c.api+"/") // 兜底 URL 与默认值一致
}
