package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// RootPath 获取项目根目录绝对路径
func RootPath() string {
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	// go run 会将编译产物放在临时目录或 build cache 中，
	// 此时 os.Executable() 返回的路径不在项目目录下，需要用编译期路径回退
	if isTempPath(exePath) {
		_, filename, _, ok := runtime.Caller(0)
		if ok {
			return filepath.Dir(filepath.Dir(filename))
		}
	}

	return findProjectRoot(filepath.Dir(exePath))
}

// rootMarkers 标识项目根目录的特征文件/目录：
// 开发环境有 go.mod 与 config.example.yaml；生产镜像只有 public/。
var rootMarkers = []string{"go.mod", "config.example.yaml", "public"}

// findProjectRoot 从 startDir 逐级向上查找包含任一 rootMarker 的目录，
// 使二进制位于根下任意深度（tmp/、runtime/tmp/ 等）都能正确定位根目录；
// 找不到时回退到 startDir 的上一级（兼容历史"二进制位于根下一层"的假设）。
func findProjectRoot(startDir string) string {
	dir := startDir
	for {
		for _, marker := range rootMarkers {
			if PathExists(filepath.Join(dir, marker)) {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Dir(startDir)
		}
		dir = parent
	}
}

// isTempPath 判断路径是否位于系统临时目录或 Go build cache 中
func isTempPath(path string) bool {
	if strings.Contains(path, os.TempDir()) {
		return true
	}
	// Go build cache（go run 产物在此）
	homeDir, err := os.UserHomeDir()
	if err == nil {
		buildCache := filepath.Join(homeDir, "Library", "Caches", "go-build")
		if strings.Contains(path, buildCache) {
			return true
		}
	}
	return false
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

// EnsureConfigFile creates the runtime configuration from the tracked example
// when a fresh installation has no editable config yet.
func EnsureConfigFile(rootPath string) error {
	configPath := filepath.Join(rootPath, "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	templatePath := filepath.Join(rootPath, "config.example.yaml")
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0600)
}
