package conf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const (
	EnvFileName        = ".env"
	EnvExampleFileName = ".env.example"
)

type AppRuntimeEnvironment struct {
	Port     string
	TimeZone string
}

func EnsureEnvFile(rootPath string) (bool, error) {
	envPath := filepath.Join(rootPath, EnvFileName)
	if _, err := os.Stat(envPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}

	examplePath := filepath.Join(rootPath, EnvExampleFileName)
	content, err := os.ReadFile(examplePath)
	if err != nil {
		if os.IsNotExist(err) {
			// 没有 .env.example 时跳过：容器内环境变量由 compose environment
			// 注入（APP_PORT/APP_TIME_ZONE），代码内置兜底，.env 非必需。
			return false, nil
		}
		return false, fmt.Errorf("read env example %q: %w", examplePath, err)
	}
	if err := os.WriteFile(envPath, content, 0600); err != nil {
		return false, fmt.Errorf("write env file %q: %w", envPath, err)
	}
	return true, nil
}

func LoadEnvFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// .env 不存在（无 .env.example 可复制）时静默跳过，行为同 EnsureEnvFile。
		return nil
	}
	if err := godotenv.Load(path); err != nil {
		return fmt.Errorf("load env file %q: %w", path, err)
	}
	return nil
}

func ResolveAppRuntimeEnvironment(getenv func(string) (string, bool)) AppRuntimeEnvironment {
	if getenv == nil {
		getenv = os.LookupEnv
	}
	return AppRuntimeEnvironment{
		Port:     envOrDefault(getenv, "APP_PORT", "9900"),
		TimeZone: envOrDefault(getenv, "APP_TIME_ZONE", "Asia/Shanghai"),
	}
}

func envOrDefault(getenv func(string) (string, bool), key, fallback string) string {
	value, ok := getenv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
