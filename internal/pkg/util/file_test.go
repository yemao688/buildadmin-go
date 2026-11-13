package util

import (
	"strings"
	"testing"
)

// 直接断言生产函数的 SVG 输出，避免复制内部实现造成伪覆盖。
func TestBuildSuffixSvg(t *testing.T) {
	svg := BuildSuffixSvg("txt", "")
	if !strings.Contains(svg, ">TXT</tspan>") {
		t.Fatalf("suffix text missing from svg: %s", svg)
	}
	if !strings.Contains(svg, "rgb(") {
		t.Fatalf("derived background color missing from svg: %s", svg)
	}
}

func TestBuildSuffixSvgUsesExplicitBackgroundAndTruncates(t *testing.T) {
	svg := BuildSuffixSvg("toooolong", "#123456")
	if !strings.Contains(svg, "fill:#123456") {
		t.Fatalf("explicit background missing from svg: %s", svg)
	}
	if !strings.Contains(svg, ">TOOO</tspan>") {
		t.Fatalf("suffix should be uppercased and truncated to 4 chars: %s", svg)
	}
}
