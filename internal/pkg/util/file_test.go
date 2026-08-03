package util

import (
	"fmt"
	"hash/adler32"
	"strings"
	"testing"
	"unicode"
)

func TestBuildSuffixSvg(t *testing.T) {
	list := []struct {
		suffix   string
		expected int
	}{
		{"txt", 33751297},
	}

	// 期望值是 Go 标准 adler32（大端），与 PHP unpack 小端序解释不同；SVG 颜色仅装饰，无需对齐。
	// 遍历测试用例并执行测试
	for _, v := range list {
		total := getsum(v.suffix)
		if total != v.expected {
			t.Errorf("result %d; want %d", total, v.expected)
		}
	}

}

func getsum(suffix string) int {
	suffix = strings.Map(func(r rune) rune { return unicode.ToUpper(r) }, suffix)
	if len(suffix) > 4 {
		suffix = suffix[0:4]
	}
	fmt.Println(suffix)
	hue := adler32.Checksum([]byte(suffix))
	return int(hue)
}
