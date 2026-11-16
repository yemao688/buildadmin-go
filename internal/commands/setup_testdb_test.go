package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buildadmin-go/internal/pkg/installer"

	"gopkg.in/yaml.v3"
)

// readSetupYAML 读取 configs 覆盖层为嵌套 map，断言用。
func readSetupYAML(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // G304: 路径由测试内 t.TempDir 构造
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var value map[string]any
	if err := yaml.Unmarshal(data, &value); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return value
}

func setupTestCaseSection() map[string]any {
	return map[string]any{
		"enabled":  true,
		"host":     "127.0.0.1",
		"port":     3306,
		"database": "buildadmin_test",
		"username": "root",
		"password": "secret",
		"charset":  "utf8mb4",
	}
}

func TestMergeMySQLTestConfigPreservesExistingKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	initial := "mysql:\n  host: 127.0.0.1\n  port: 3306\n  database: buildadmin\n  username: root\n  password: secret\n  prefix: ba_\ntoken:\n  key: generated-token\napp:\n  env: release\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := mergeMySQLTestConfig(path, setupTestCaseSection()); err != nil {
		t.Fatalf("mergeMySQLTestConfig() error = %v", err)
	}

	got := readSetupYAML(t, path)
	mysqlTest, ok := got["mysql_test"].(map[string]any)
	if !ok {
		t.Fatalf("mysql_test section missing: %#v", got)
	}
	if mysqlTest["enabled"] != true {
		t.Fatalf("mysql_test.enabled = %#v, want true", mysqlTest["enabled"])
	}
	for key, want := range map[string]any{
		"host":     "127.0.0.1",
		"database": "buildadmin_test",
		"username": "root",
		"password": "secret",
		"charset":  "utf8mb4",
	} {
		if mysqlTest[key] != want {
			t.Fatalf("mysql_test.%s = %#v, want %#v", key, mysqlTest[key], want)
		}
	}
	if mysqlTest["port"] != 3306 {
		t.Fatalf("mysql_test.port = %#v, want 3306", mysqlTest["port"])
	}

	token, ok := got["token"].(map[string]any)
	if !ok || token["key"] != "generated-token" {
		t.Fatalf("token.key was not preserved: %#v", got["token"])
	}
	app, ok := got["app"].(map[string]any)
	if !ok || app["env"] != "release" {
		t.Fatalf("app.env was not preserved: %#v", got["app"])
	}
	mysql, ok := got["mysql"].(map[string]any)
	if !ok || mysql["prefix"] != "ba_" || mysql["database"] != "buildadmin" {
		t.Fatalf("mysql section was not preserved: %#v", got["mysql"])
	}
}

func TestMergeMySQLTestConfigIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("mysql:\n  host: 127.0.0.1\ntoken:\n  key: token\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	section := setupTestCaseSection()
	if err := mergeMySQLTestConfig(path, section); err != nil {
		t.Fatalf("first mergeMySQLTestConfig() error = %v", err)
	}
	first, err := os.ReadFile(path) //nolint:gosec // G304: 测试临时路径
	if err != nil {
		t.Fatal(err)
	}
	if err := mergeMySQLTestConfig(path, section); err != nil {
		t.Fatalf("second mergeMySQLTestConfig() error = %v", err)
	}
	second, err := os.ReadFile(path) //nolint:gosec // G304: 测试临时路径
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("second merge changed the file:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestMergeMySQLTestConfigCreatesMissingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := mergeMySQLTestConfig(path, setupTestCaseSection()); err != nil {
		t.Fatalf("mergeMySQLTestConfig() error = %v", err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config.yaml was not created: %v", err)
	}
	if perm := stat.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config.yaml mode = %o, want 600", perm)
	}
	got := readSetupYAML(t, path)
	mysqlTest, ok := got["mysql_test"].(map[string]any)
	if !ok || mysqlTest["enabled"] != true || mysqlTest["database"] != "buildadmin_test" {
		t.Fatalf("created mysql_test section = %#v", got["mysql_test"])
	}
}

// TestPrepareSetupTestDatabaseGrantFailureKeepsInstall 覆盖 GRANT 权限不足的降级
// 路径：配置仍写入 enabled:true，并打印可直接执行的手工 SQL。
func TestPrepareSetupTestDatabaseGrantFailureKeepsInstall(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, []byte("mysql:\n  host: 127.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	database := installer.Database{
		Database: "buildadmin",
		Hostname: "127.0.0.1",
		Hostport: "3306",
		Username: "root",
		Password: "secret",
		Prefix:   "ba_",
	}
	created := ""
	runner := setupRunner{
		configPath: configPath,
		out:        &bytes.Buffer{},
		deps: setupDependencies{
			createTestDatabase: func(_ installer.Database, name string) error {
				created = name
				return nil
			},
			grantTestDatabase: func(installer.Database, string) error {
				return os.ErrPermission
			},
			mergeTestConfig: mergeMySQLTestConfig,
		},
	}

	runner.prepareSetupTestDatabase(
		setupOptions{withTestDB: true, yes: true},
		setupInput{database: database},
		newSetupConsole(strings.NewReader(""), runner.out),
	)

	if created != "buildadmin_test" {
		t.Fatalf("created test database = %q, want buildadmin_test", created)
	}
	got := readSetupYAML(t, configPath)
	mysqlTest, ok := got["mysql_test"].(map[string]any)
	if !ok || mysqlTest["enabled"] != true || mysqlTest["database"] != "buildadmin_test" {
		t.Fatalf("mysql_test section = %#v", got["mysql_test"])
	}
	output := runner.out.(*bytes.Buffer).String()
	if !strings.Contains(output, "GRANT ALL PRIVILEGES ON `buildadmin%`.* TO 'root'@'%';") {
		t.Fatalf("manual GRANT statement missing from output: %q", output)
	}
}

// TestPrepareSetupTestDatabaseCreateFailureSkipsConfig 保证建库失败时不写出
// 指向不存在库的 enabled:true 配置。
func TestPrepareSetupTestDatabaseCreateFailureSkipsConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(configPath, []byte("mysql:\n  host: 127.0.0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := setupRunner{
		configPath: configPath,
		out:        &bytes.Buffer{},
		deps: setupDependencies{
			createTestDatabase: func(installer.Database, string) error { return os.ErrPermission },
			grantTestDatabase:  func(installer.Database, string) error { return nil },
			mergeTestConfig:    mergeMySQLTestConfig,
		},
	}
	database := installer.Database{Database: "buildadmin", Hostname: "127.0.0.1", Hostport: "3306", Username: "root", Password: "secret"}

	runner.prepareSetupTestDatabase(
		setupOptions{withTestDB: true, yes: true},
		setupInput{database: database},
		newSetupConsole(strings.NewReader(""), runner.out),
	)

	got := readSetupYAML(t, configPath)
	if _, ok := got["mysql_test"]; ok {
		t.Fatalf("mysql_test must not be written when the database creation failed: %#v", got["mysql_test"])
	}
	if !strings.Contains(runner.out.(*bytes.Buffer).String(), "CREATE DATABASE IF NOT EXISTS `buildadmin_test`") {
		t.Fatalf("manual CREATE DATABASE statement missing from output: %q", runner.out.(*bytes.Buffer).String())
	}
}
