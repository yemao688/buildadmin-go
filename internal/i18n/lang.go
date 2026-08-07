package i18n

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

// NormalizeLang 将外部语言标识规范化到后端本地化 pack key
// （ginI18n localizer map key 为 zh / zh-Hant / en，见 NewBundleCfg）。
// 前端传 zh-cn 需显式映射到 zh，不再依赖 localizer map 未命中的
// 默认兜底巧合。未知语言原样返回，便于下游扩展语言时在此追加映射。
func NormalizeLang(lng string) string {
	switch strings.ToLower(lng) {
	case "zh-cn":
		return "zh"
	case "zh-hant", "zh-tw":
		return "zh-Hant"
	case "en":
		return "en"
	default:
		return lng
	}
}

// langContextKey 是当前请求已解析语言（规范化 pack key）的存储键，同时写入
// gin context（c.Keys）与请求 context（c.Request.Context()）。独立字符串
// 避免与 ginI18n 的 "i18n"（localizer 实例）等既有键冲突。
const langContextKey = "request.lang"

// LangFromContext 读取当前请求已解析的语言（resolveRequestLang 的缓存结果，
// 规范化 pack key，如 zh / zh-Hant / en）。ctx 可以是 *gin.Context、其派生
// context（context.WithValue 链），或经 SetLangToContext 写入的请求 context。
// 未设置或为空值时返回 ok=false，调用方自行回退。
func LangFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	lang, ok := ctx.Value(langContextKey).(string)
	if !ok || lang == "" {
		return "", false
	}
	return lang, true
}

// SetLangToContext 将请求语言写入 gin context 与请求 context，供同一请求内
// 后续翻译调用（ginI18n getLngHandler 去重）与 DB 翻译串联
// （country.Service.GetByRequest）复用。
func SetLangToContext(c *gin.Context, lang string) {
	if c == nil {
		return
	}
	c.Set(langContextKey, lang)
	if c.Request != nil {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), langContextKey, lang))
	}
}
