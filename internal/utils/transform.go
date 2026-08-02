package utils

import (
	"regexp"
	"strconv"
	"strings"
)

func CamelToSnake(input string) string {
	// 使用正则表达式匹配大写字母
	re := regexp.MustCompile(`([A-Z])`)
	// 替换所有匹配的大写字母，在它们之前加上下划线，并将它们转换为小写
	return re.ReplaceAllStringFunc(input, func(s string) string {
		return "_" + strings.ToLower(s)
	})
}

func SnakeToCamel(input string, ucfirst bool) string {
	// 使用正则表达式匹配下划线后面跟的小写字母
	re := regexp.MustCompile(`_([a-z])`)
	// 替换所有匹配的下划线+字母，在它们之后将字母转换为大写
	name := re.ReplaceAllStringFunc(input, func(s string) string {
		return strings.ToUpper(s[1:])
	})

	if ucfirst {
		return strings.ToUpper(name[:1]) + name[1:]
	}
	return strings.ToLower(name[:1]) + name[1:]
}

// StrAttrToArray 将字符串属性列表转为map
// attr  属性，一行一个，无需引号，比如：class=input-class
func StrAttrToArray(attr string) map[string]interface{} {
	if attr == "" {
		return make(map[string]interface{})
	}

	attrLines := strings.Split(strings.ReplaceAll(attr, "\r\n", "\n"), "\n")
	attrTemp := make(map[string]interface{})

	for _, item := range attrLines {
		parts := strings.Split(item, "=")
		if len(parts) >= 2 {
			var attrVal any

			attrVal = parts[1]
			if parts[1] == "false" || parts[1] == "true" {
				attrVal = !strings.EqualFold(parts[1], "false")
			} else if val, err := strconv.ParseFloat(parts[1], 64); err == nil {
				attrVal = val
			}

			attrKeyParts := strings.Split(parts[0], ".")
			if len(attrKeyParts) == 2 {
				attrTemp[attrKeyParts[0]] = map[string]interface{}{attrKeyParts[1]: attrVal}
			} else {
				attrTemp[parts[0]] = attrVal
			}
		}
	}

	return attrTemp
}
