package utils

import (
	"os"
	"path/filepath"
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
	configPath := filepath.Join(rootPath, "conf", "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	templatePath := filepath.Join(rootPath, "conf", "config.example.yaml")
	data, err := os.ReadFile(templatePath)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0600)
}
