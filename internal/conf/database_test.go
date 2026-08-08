package conf

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDatabasePoolConfigYAMLMapping 验证连接池调优配置项与 YAML 键的映射
// （mapstructure），防止键拼写错误或字段重命名后配置静默失效。
func TestDatabasePoolConfigYAMLMapping(t *testing.T) {
	root := t.TempDir()
	defaultsPath := filepath.Join(root, DefaultsFileName)
	yaml := "mysql:\n" +
		"  max_idle_conns: 50\n" +
		"  max_open_conns: 300\n" +
		"  max_idle_time: 300\n" +
		"  conn_max_lifetime: 1800\n"
	if err := os.WriteFile(defaultsPath, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}

	v, deadKeys, err := LoadLayeredConfig(defaultsPath, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(deadKeys) != 0 {
		t.Fatalf("dead keys = %#v, want none", deadKeys)
	}

	var configuration Configuration
	if err := v.Unmarshal(&configuration); err != nil {
		t.Fatal(err)
	}
	d := configuration.Database
	if d.MaxIdleConns != 50 || d.MaxOpenConns != 300 || d.MaxIdleTime != 300 || d.ConnMaxLifetime != 1800 {
		t.Fatalf("database pool config = %+v, want max_idle_conns=50 max_open_conns=300 max_idle_time=300 conn_max_lifetime=1800", d)
	}
}
