package utils

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
		{"txt", 16843522},
	}

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
