package i18n

import "strings"

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
