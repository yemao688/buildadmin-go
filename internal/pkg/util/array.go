package util

import (
	"fmt"
	"strconv"
)

func RemoveStrDuplicates(arr []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, value := range arr {
		if _, ok := seen[value]; !ok {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func AtoiArr(arr []string) ([]int32, error) {
	result := []int32{}
	for _, v := range arr {
		num, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to convert '%s' to int32: %w", v, err)
		}
		result = append(result, int32(num))
	}
	return result, nil
}
