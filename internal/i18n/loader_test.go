package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"buildadmin-go/internal/pkg/util"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// TestEmbeddedBundleResolvesKnownKeys proves the locale packs embedded in the
// binary resolve through the same middleware path the application uses.
func TestEmbeddedBundleResolvesKnownKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginI18n.Localize(ginI18n.WithBundle(NewBundleCfg()), ginI18n.WithGetLngHandle(
		func(c *gin.Context, defaultLng string) string {
			// 与生产一致：think-lang 经 NormalizeLang 规范化到 pack key
			// （zh-hant → zh-Hant），否则小写 header 会因 localizer map
			// key 未命中而回退默认语言。
			if lng := c.Request.Header.Get("think-lang"); lng != "" {
				return NormalizeLang(lng)
			}
			return defaultLng
		},
	)))
	r.GET("/msg", func(c *gin.Context) {
		c.String(http.StatusOK, util.Lang(c, "Super administrator", nil))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/msg", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, "super administrator", w.Body.String())

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/msg", nil)
	req.Header.Set("think-lang", "en")
	r.ServeHTTP(w, req)
	require.Equal(t, "super administrator", w.Body.String())

	// zh-Hant 包已补全：think-lang=zh-hant 应返回繁体而非回退默认语言
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/msg", nil)
	req.Header.Set("think-lang", "zh-hant")
	r.ServeHTTP(w, req)
	require.Equal(t, "超級管理員", w.Body.String())
}

// TestAcceptLanguageDerivedFromLocaleFiles 验证 AcceptLanguage 与 embed 的
// locales 目录 yaml 文件一一对应（语言包即事实源），新增/删除语言包后无需
// 同步接线。
func TestAcceptLanguageDerivedFromLocaleFiles(t *testing.T) {
	entries, err := localesFS.ReadDir("locales")
	require.NoError(t, err)
	var packFiles []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > len(".yaml") && e.Name()[len(e.Name())-len(".yaml"):] == ".yaml" {
			packFiles = append(packFiles, e.Name())
		}
	}
	require.NotEmpty(t, packFiles, "locales 目录应至少有一个语言包")

	tags := NewBundleCfg().AcceptLanguage
	require.Len(t, tags, len(packFiles), "AcceptLanguage 应与语言包文件一一对应")
	seen := map[string]bool{}
	for _, tag := range tags {
		seen[tag.String()] = true
	}
	for _, f := range packFiles {
		want := f[:len(f)-len(".yaml")]
		require.True(t, seen[want], "AcceptLanguage 缺少语言包 %q", want)
	}
}

// TestNewLocalePackAutoAccepted 回归：新增语言包（fr.yaml）后，think-lang=fr
// 应返回法语而非静默回退默认语言 zh。模拟 embed FS 外再放一个法语包，走
// 与生产相同的 ginI18n 中间件通路。
func TestNewLocalePackAutoAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fsys := fstest.MapFS{
		"locales/zh-CN.yaml": {Data: []byte("Super administrator: 超级管理员\n")},
		"locales/fr.yaml":    {Data: []byte("Super administrator: super administrateur\n")},
	}
	tags, err := acceptLanguageFromFS(fsys)
	require.NoError(t, err)
	require.Equal(t, []language.Tag{language.Make("fr"), language.Make("zh-CN")}, tags)

	r := gin.New()
	r.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         "locales",
		AcceptLanguage:   tags,
		DefaultLanguage:  language.Make("zh-CN"),
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
		Loader:           ginI18n.LoaderFunc(fsys.ReadFile),
	}), ginI18n.WithGetLngHandle(
		func(c *gin.Context, defaultLng string) string {
			if lng := c.Request.Header.Get("think-lang"); lng != "" {
				return lng
			}
			return defaultLng
		},
	)))
	r.GET("/msg", func(c *gin.Context) {
		c.String(http.StatusOK, util.Lang(c, "Super administrator", nil))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/msg", nil)
	req.Header.Set("think-lang", "fr")
	r.ServeHTTP(w, req)
	require.Equal(t, "super administrateur", w.Body.String())
}
