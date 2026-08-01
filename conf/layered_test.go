package conf

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMergeConfigMapsDeepMergeAndListReplacement(t *testing.T) {
	base := map[string]any{
		"app":     map[string]any{"env": "debug", "port": 9900},
		"servers": []any{"default"},
	}
	override := map[string]any{
		"app":     map[string]any{"port": 8080},
		"servers": []any{"custom"},
	}

	got := MergeConfigMaps(base, override)
	want := map[string]any{
		"app":     map[string]any{"env": "debug", "port": 8080},
		"servers": []any{"custom"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged config = %#v, want %#v", got, want)
	}
	if base["app"].(map[string]any)["port"] != 9900 {
		t.Fatal("merge mutated base config")
	}
}

func TestFindDeadKeys(t *testing.T) {
	defaults := map[string]any{
		"app":   map[string]any{"env": "debug"},
		"mysql": map[string]any{"host": "127.0.0.1"},
	}
	overrides := map[string]any{
		"app":    map[string]any{"env": "release", "removed": true},
		"mysql":  map[string]any{"host": "db"},
		"legacy": map[string]any{"key": "value"},
	}

	got := FindDeadKeys(defaults, overrides)
	want := []string{"app.removed", "legacy.key"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dead keys = %#v, want %#v", got, want)
	}
}

func TestLoadLayeredConfigSparseOverride(t *testing.T) {
	root := t.TempDir()
	defaultsPath := filepath.Join(root, DefaultsFileName)
	overridePath := filepath.Join(root, "config.yaml")
	defaults := []byte("app:\n  env: debug\n  port: 9900\nmysql:\n  host: 127.0.0.1\n  port: 3306\n  database: buildadmin\nmysql_test:\n  enabled: false\n")
	override := []byte("mysql:\n  host: db.internal\n  database: custom\n")
	if err := os.WriteFile(defaultsPath, defaults, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overridePath, override, 0600); err != nil {
		t.Fatal(err)
	}

	v, deadKeys, err := LoadLayeredConfig(defaultsPath, overridePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(deadKeys) != 0 {
		t.Fatalf("dead keys = %#v, want none", deadKeys)
	}
	if got := v.GetString("mysql.host"); got != "db.internal" {
		t.Errorf("mysql.host = %q, want db.internal", got)
	}
	if got := v.GetString("app.env"); got != "debug" {
		t.Errorf("app.env = %q, want base default debug", got)
	}
	if got := v.GetInt("mysql.port"); got != 3306 {
		t.Errorf("mysql.port = %d, want base default 3306", got)
	}
	if got := v.GetBool("mysql_test.enabled"); got {
		t.Error("mysql_test.enabled unexpectedly changed")
	}
}

func TestLoadLayeredConfigUnmarshalsWithoutRemovedAppName(t *testing.T) {
	root := t.TempDir()
	defaultsPath := filepath.Join(root, DefaultsFileName)
	if err := os.WriteFile(defaultsPath, []byte("app:\n  env: debug\nmysql:\n  host: 127.0.0.1\n"), 0600); err != nil {
		t.Fatal(err)
	}

	v, deadKeys, err := LoadLayeredConfig(defaultsPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(deadKeys) != 0 {
		t.Fatalf("dead keys = %#v", deadKeys)
	}
	var configuration Configuration
	if err := v.Unmarshal(&configuration); err != nil {
		t.Fatal(err)
	}
	if configuration.App.Env != "debug" {
		t.Fatalf("app.env = %q, want debug", configuration.App.Env)
	}
	if configuration.App.Port != "" || configuration.App.TimeZone != "" {
		t.Fatalf("YAML unexpectedly supplied runtime values: %#v", configuration.App)
	}
}

func TestWriteConfigOverridesPreservesExistingOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  env: release\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteConfigOverrides(path, map[string]any{
		"mysql": map[string]any{"host": "db.internal"},
		"token": map[string]any{"key": "generated"},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := readYAMLMap(path)
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeConfigMaps(nil, data)
	if merged["app"].(map[string]any)["env"] != "release" {
		t.Error("existing app override was lost")
	}
	if merged["mysql"].(map[string]any)["host"] != "db.internal" {
		t.Error("installation mysql override was not written")
	}
}
