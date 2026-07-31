package testutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeMySQLTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	defaultsPath := filepath.Join(dir, "config.defaults.yaml")
	if err := os.WriteFile(defaultsPath, []byte("app:\n  env: debug\n"), 0600); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func TestLoadMySQLTestConfig(t *testing.T) {
	t.Run("config.yaml missing", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "config.yaml")
		_, err := loadMySQLTestConfig(configPath)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("loadMySQLTestConfig() error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("mysql_test missing", func(t *testing.T) {
		configPath := writeMySQLTestConfig(t, "app:\n  env: debug\n")
		_, err := loadMySQLTestConfig(configPath)
		if !errors.Is(err, errMysqlTestNotConfigured) {
			t.Fatalf("loadMySQLTestConfig() error = %v, want errMysqlTestNotConfigured", err)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		configPath := writeMySQLTestConfig(t, "mysql_test:\n  enabled: false\n")
		configuration, err := loadMySQLTestConfig(configPath)
		if err != nil {
			t.Fatalf("loadMySQLTestConfig() error = %v", err)
		}
		if configuration.MysqlTest.Enabled {
			t.Fatal("MysqlTest.Enabled = true, want false")
		}
	})

	t.Run("parses enabled config", func(t *testing.T) {
		configPath := writeMySQLTestConfig(t, `mysql_test:
  enabled: true
  host: 'mysql.test'
  port: 3307
  database: 'buildadmin_test'
  username: 'buildadmin_go'
  password: 'secret'
  charset: 'utf8mb4'
`)
		configuration, err := loadMySQLTestConfig(configPath)
		if err != nil {
			t.Fatalf("loadMySQLTestConfig() error = %v", err)
		}
		got := configuration.MysqlTest
		if !got.Enabled || got.Host != "mysql.test" || got.Port != 3307 || got.Database != "buildadmin_test" || got.UserName != "buildadmin_go" || got.Password != "secret" || got.Charset != "utf8mb4" {
			t.Fatalf("MysqlTest = %+v, want all configured values", got)
		}
	})
}
