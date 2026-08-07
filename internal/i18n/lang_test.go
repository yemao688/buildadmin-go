package i18n

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeLang(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"zh-cn 映射到 zh pack key", "zh-cn", "zh"},
		{"zh-CN 大写映射到 zh", "zh-CN", "zh"},
		{"zh-hant 映射到 zh-Hant", "zh-hant", "zh-Hant"},
		{"zh-Hant 原格式映射到 zh-Hant", "zh-Hant", "zh-Hant"},
		{"zh-tw 映射到 zh-Hant", "zh-tw", "zh-Hant"},
		{"zh-TW 大写映射到 zh-Hant", "zh-TW", "zh-Hant"},
		{"en 原样", "en", "en"},
		{"EN 大写归一为 en", "EN", "en"},
		{"未知语言原样返回", "ja", "ja"},
		{"空字符串原样返回", "", ""},
		{"zh 本身原样返回", "zh", "zh"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, NormalizeLang(tt.in))
		})
	}
}
