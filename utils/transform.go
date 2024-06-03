package utils

import (
	"strconv"
	"strings"
)

func SnakeToCamel(snakeCase string) string {
	words := strings.Split(snakeCase, "_")
	for i, word := range words {
		if i == 0 {
			continue // Skip the first word (no need to capitalize it)
		}
		// Capitalize the first letter of the word
		words[i] = strings.Title(word)
	}
	return strings.Join(words, "") // Merge the words back into a single string
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
