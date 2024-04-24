package utils

import "strings"

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
