// Package i18n hosts the backend locale packs and the bundle configuration
// for the gin-contrib/i18n middleware. Packs are embedded into the binary:
// the runtime needs no on-disk locale directory.
package i18n

import (
	"embed"

	ginI18n "github.com/gin-contrib/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// LocalesDir is the tracked locale pack directory (the source of the embedded
// packs), relative to the repository root. Edit packs here and rebuild.
const LocalesDir = "internal/i18n/locales"

//go:embed locales
var localesFS embed.FS

// NewBundleCfg returns the gin-contrib/i18n bundle configuration backed by the
// embedded YAML locale packs (en.yaml, zh.yaml, zh-Hant.yaml).
func NewBundleCfg() *ginI18n.BundleCfg {
	return &ginI18n.BundleCfg{
		RootPath:         "locales",
		AcceptLanguage:   []language.Tag{language.Chinese, language.TraditionalChinese, language.English},
		DefaultLanguage:  language.Chinese,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
		Loader:           &ginI18n.EmbedLoader{FS: localesFS},
	}
}
