// Package i18n hosts the backend locale packs and the bundle configuration
// for the gin-contrib/i18n middleware. Packs are embedded into the binary:
// the runtime needs no on-disk locale directory.
package i18n

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	ginI18n "github.com/gin-contrib/i18n"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// LocalesDir is the tracked locale pack directory (the source of the embedded
// packs), relative to the repository root. Edit packs here and rebuild.
// 文件名必须是规范 BCP 47 tag（en.yaml、zh-CN.yaml、zh-Hant.yaml；不要用
// zh-cn.yaml 等非规范写法——Parse 会规范化大小写，导致加载路径与文件名
// 不匹配而启动 panic）。对外契约（country_language.lan、前端 lang）仍用
// zh-cn 小写，经 NormalizeLang 统一规范化为 zh-CN。
const LocalesDir = "internal/i18n/locales"

//go:embed locales
var localesFS embed.FS

// acceptLanguageFromFS 从 locale pack 目录派生 AcceptLanguage tag 列表：
// 语言包即 AcceptLanguage 的事实源——新增 yaml 后无需手工接线（与前端 glob
// 全量自动发现对齐，避免 think-lang 请求因未同步列表而静默回退默认语言）。
// 默认语言（DefaultLanguage 与 router 兜底）是独立配置，不随语言包自动推导。
// tag 字符串与文件名一致（en.yaml -> en、zh-CN.yaml -> zh-CN），与
// gin-contrib/i18n 的加载路径（locales/<tag.String()>.yaml）天然自洽。
func acceptLanguageFromFS(fsys fs.FS) ([]language.Tag, error) {
	entries, err := fs.ReadDir(fsys, "locales")
	if err != nil {
		return nil, err
	}
	tags := make([]language.Tag, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		tag, err := language.Parse(strings.TrimSuffix(e.Name(), ".yaml"))
		if err != nil {
			return nil, fmt.Errorf("locale pack %q: %w", e.Name(), err)
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

// NewBundleCfg returns the gin-contrib/i18n bundle configuration backed by the
// embedded YAML locale packs. AcceptLanguage is derived from the pack
// directory instead of a hardcoded list, so adding a new locale (e.g. fr.yaml)
// wires it up automatically.
func NewBundleCfg() *ginI18n.BundleCfg {
	tags, err := acceptLanguageFromFS(localesFS)
	if err != nil {
		// embed 编译期固定，仅 pack 文件命名非法（非 BCP47 tag）时触发，
		// 属构建期错误，启动即暴露而非静默回退。
		panic(err)
	}
	return &ginI18n.BundleCfg{
		RootPath:         "locales",
		AcceptLanguage:   tags,
		DefaultLanguage:  language.English,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
		Loader:           &ginI18n.EmbedLoader{FS: localesFS},
	}
}
