package utils

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// RootPath 获取项目根目录绝对路径
func RootPath() string {
	var rootDir string

	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	rootDir = filepath.Dir(filepath.Dir(exePath))

	tmpDir := os.TempDir()
	if strings.Contains(exePath, tmpDir) {
		_, filename, _, ok := runtime.Caller(0)
		if ok {
			rootDir = filepath.Dir(filepath.Dir(filename))
		}
	}

	return rootDir
}

// 没有就取默认地址
func DefaultUrl(relativeUrl string, defaultUrl string) string {
	if relativeUrl == "" {
		return defaultUrl
	}
	return relativeUrl
}

// 获取资源完整url地址；若安装了云存储或 config  配置了CdnUrl，则自动使用对应的CdnUrl
func FullUrl(relativeUrl string, cdn string, domain string, defaultUrl string) string {
	h := cdn
	if cdn == "" {
		h = domain
	}

	if relativeUrl == "" {
		relativeUrl = defaultUrl
	}

	if relativeUrl == "" {
		return h
	}

	regex := regexp.MustCompile(`^((?:[a-z]+:)?\/\/|data:image\/)(.*)`)
	ok, _ := regexp.MatchString(`^http(s)?:\/\/`, relativeUrl)
	if ok || regex.MatchString(relativeUrl) {
		return relativeUrl
	}
	return h + relativeUrl
}
