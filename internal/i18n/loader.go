// Package i18n hosts the backend locale packs and the bundle configuration
// for the gin-contrib/i18n middleware.
package i18n

import (
	"path/filepath"

	ginI18n "github.com/gin-contrib/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// LocalesDir is the tracked locale pack directory, relative to the repository root.
const LocalesDir = "internal/i18n/locales"

// NewBundleCfg returns the gin-contrib/i18n bundle configuration that loads
// the YAML locale packs (en.yaml, zh.yaml, zh-Hant.yaml) from
// <rootPath>/internal/i18n/locales.
func NewBundleCfg(rootPath string) *ginI18n.BundleCfg {
	return &ginI18n.BundleCfg{
		RootPath:         filepath.Join(rootPath, LocalesDir),
		AcceptLanguage:   []language.Tag{language.Chinese, language.TraditionalChinese, language.English},
		DefaultLanguage:  language.Chinese,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
	}
}
