package version

import (
	"go-build-admin/app/pkg/terminal"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// 比较两个版本号
func Compare(v1, v2 string) bool {
	if v2 == "" {
		return false
	}

	if strings.ToLower(string(v1[0])) == "v" {
		v1 = v1[1:]
	}

	if strings.ToLower(string(v2[0])) == "v" {
		v2 = v2[1:]
	}

	if v1 == "*" || v1 == v2 {
		return true
	}

	if strings.Contains(v1, "-") {
		v1 = strings.Split(v1, "-")[0]
	}

	if strings.Contains(v2, "-") {
		v2 = strings.Split(v2, "-")[0]
	}

	arr1 := strings.Split(v1, ".")
	arr2 := strings.Split(v2, ".")

	for i := 0; i < len(arr1); i++ {
		if i >= len(arr2) {
			break
		}

		num1, _ := strconv.Atoi(arr1[i])
		num2, _ := strconv.Atoi(arr2[i])

		if num1 == num2 {
			continue
		}

		if num1 > num2 {
			return false
		}

		if num1 < num2 {
			return true
		}
	}

	if len(arr1) != len(arr2) {
		return !(len(arr1) > len(arr2))
	}
	return false
}

func CheckDigitalVersion(version string) bool {
	if version == "" {
		return false
	}

	if strings.ToLower(string(version[0])) == "v" {
		version = version[1:]
	}

	// 规则1: 检查是否有两个以上的连续点
	rule1 := regexp.MustCompile(`\.{2,10}`)
	if rule1.MatchString(version) {
		return false // 如果找到连续的点，则返回false
	}

	// 规则2: 检查是否符合常见版本号格式
	rule2 := regexp.MustCompile(`^\d+(\.\d+){0,10}$`)
	return rule2.MatchString(version) // 直接返回规则2的匹配结果
}

// 获取依赖版本号
func GetVersion(terminalHelper *terminal.Terminal, name string) string {
	if name == "cnpm" {
		execOut, ok := terminalHelper.GetCommandOutput("version.cnpm")
		if ok && execOut != "" {
			pattern := regexp.MustCompile(`cnpm@(.+?) \(`)
			result := pattern.FindStringSubmatch(execOut)
			if len(result) > 1 {
				return result[1]
			}
		}
		return ""
	}

	if slices.Contains([]string{"npm", "yarn", "pnpm", "node"}, name) {
		execOut, ok := terminalHelper.GetCommandOutput("version." + name)
		if ok && execOut != "" {
			if strings.Contains(execOut, "npm WARN") {
				pattern := regexp.MustCompile(`\d+(\.\d+){0,2}`)
				result := pattern.FindStringSubmatch(execOut)
				if len(result) > 0 && CheckDigitalVersion(result[0]) {
					return result[0]
				}
			}

			lines := strings.Split(execOut, "\r\n")
			lines = append(lines, strings.Split(execOut, "\r")...)
			lines = append(lines, strings.Split(execOut, "\n")...)
			uniqueLines := unique(lines)

			// 检测两行，第一行可能会是个警告消息
			for i := 0; i < 2; i++ {
				if len(uniqueLines) > i && CheckDigitalVersion(uniqueLines[i]) {
					return uniqueLines[i]
				}
			}
		}
	}
	return ""
}

func unique(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
