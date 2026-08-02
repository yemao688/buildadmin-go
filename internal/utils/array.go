package utils

import (
	"fmt"
	"strconv"
)

func RemoveDuplicates(arr []int32) []int32 {
	seen := make(map[int32]bool)
	result := []int32{}

	for _, value := range arr {
		if _, ok := seen[value]; !ok {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

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

func ItoaArr(arr []int32) []string {
	result := []string{}
	for _, v := range arr {
		result = append(result, strconv.FormatInt(int64(v), 10))
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
