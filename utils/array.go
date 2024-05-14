package utils

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

func ContainsString(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}
