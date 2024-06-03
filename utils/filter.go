package utils

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

func Filter(input string) string {
	// 去除字符串两端空格
	input = strings.TrimSpace(input)
	// 特殊字符转实体
	return html.EscapeString(input)
}

func StripTags(s string) string {
	re := regexp.MustCompile(`<.*?>`)
	return re.ReplaceAllString(s, "")
}

func Htmlspecialchars(s string) string {
	var result strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			result.WriteString("&amp;")
		case '<':
			result.WriteString("&lt;")
		case '>':
			result.WriteString("&gt;")
		case '"':
			result.WriteString("&quot;")
		case '\'':
			result.WriteString("&apos;")
		default:
			if utf8.RuneLen(r) > 1 {
				result.WriteRune(r)
			} else {
				result.WriteString(string(r))
			}
		}
	}
	return result.String()
}
