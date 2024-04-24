package utils

import (
	"strings"

	"golang.org/x/net/html"
)

func Filter(input string) string {
	// 去除字符串两端空格
	input = strings.TrimSpace(input)
	// 特殊字符转实体
	return html.EscapeString(input)
}
