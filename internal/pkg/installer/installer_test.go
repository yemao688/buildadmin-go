package installer

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"buildadmin-go/internal/conf"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDatabaseDSN(t *testing.T) {
	cfg := Database{
		Database: "buildadmin",
		Hostname: "db.example.test",
		Hostport: "3307",
		Username: "installer",
		Password: "p@ss word",
	}

	require.Equal(t,
		"installer:p@ss word@tcp(db.example.test:3307)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local",
		databaseDSN(cfg),
	)
	require.Equal(t,
		"installer:p@ss word@tcp(db.example.test:3307)/?charset=utf8mb4&parseTime=True&loc=Local",
		serverDSN(cfg),
	)
}

func TestIsComplete(t *testing.T) {
	root := t.TempDir()
	public := filepath.Join(root, "public")
	require.NoError(t, os.Mkdir(public, 0755))

	require.False(t, IsComplete(root))
	lockPath := filepath.Join(public, LockFileName)
	require.NoError(t, os.WriteFile(lockPath, []byte("in-progress"), 0644))
	require.False(t, IsComplete(root))
	require.NoError(t, os.WriteFile(lockPath, []byte(InstallationCompletionMark), 0644))
	require.True(t, IsComplete(root))

	// 仅凭 configs/config.yaml 不视为已安装：安装器只认 install.lock
	// （向导在环境检查步骤即写入配置，配置存在不代表安装完成）。
	configRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(configRoot, "configs"), 0755))
	require.False(t, IsComplete(configRoot))
	require.NoError(t, os.WriteFile(filepath.Join(configRoot, "configs", "config.yaml"), []byte("mysql:\n    host: 127.0.0.1\n"), 0644))
	require.False(t, IsComplete(configRoot))

	require.NoError(t, WriteCompletionLock(root))
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	require.Equal(t, []byte(InstallationCompletionMark), content)
}

func TestWriteBaseConfig(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "configs")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	configPath := filepath.Join(configDir, "config.yaml")
	cfg := Database{
		Database: "buildadmin",
		Hostname: "127.0.0.1",
		Hostport: "3308",
		Username: "root",
		Password: "secret",
		Prefix:   "ba_",
	}

	require.NoError(t, WriteBaseConfig(configPath, cfg, "token-value"))
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, yaml.Unmarshal(data, &got))
	require.Equal(t, map[string]any{
		"host":     "127.0.0.1",
		"port":     3308,
		"database": "buildadmin",
		"username": "root",
		"password": "secret",
		"prefix":   "ba_",
	}, got["mysql"])
	require.Equal(t, map[string]any{"key": "token-value"}, got["token"])
}

func TestWriteBaseConfigRejectsInvalidPort(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "configs")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	err := WriteBaseConfig(filepath.Join(configDir, "config.yaml"), Database{Hostport: "not-a-port"}, "token")
	require.Error(t, err)
}

func TestGenerateTokenKey(t *testing.T) {
	key := GenerateTokenKey()
	require.Len(t, key, 32)
	for _, char := range key {
		require.Contains(t, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", string(char))
	}
}

func TestPoolSettings(t *testing.T) {
	// 零值配置回退默认，与 config.defaults.yaml / 运行期 infra/db 语义一致。
	maxIdle, maxOpen, lifetime, idleTime := poolSettings(conf.Database{})
	require.Equal(t, 50, maxIdle)
	require.Equal(t, 300, maxOpen)
	require.Equal(t, 1800*time.Second, lifetime)
	require.Equal(t, 300*time.Second, idleTime)

	// 配置值生效。
	maxIdle, maxOpen, lifetime, idleTime = poolSettings(conf.Database{MaxIdleConns: 7, MaxOpenConns: 9, ConnMaxLifetime: 60, MaxIdleTime: 15})
	require.Equal(t, 7, maxIdle)
	require.Equal(t, 9, maxOpen)
	require.Equal(t, 60*time.Second, lifetime)
	require.Equal(t, 15*time.Second, idleTime)

	// 负值同样回退。
	_, _, lifetime, idleTime = poolSettings(conf.Database{ConnMaxLifetime: -1, MaxIdleTime: -1})
	require.Equal(t, 1800*time.Second, lifetime)
	require.Equal(t, 300*time.Second, idleTime)
}

// TestLoadPoolConfig 覆盖分层配置读取：基座 config.defaults.yaml 提供连接池
// 值；稀疏覆盖层 config.yaml 存在时合并生效；覆盖层不存在（全新安装早期，
// 安装器尚未写入 config）时退化为仅基座。
func TestLoadPoolConfig(t *testing.T) {
	root := t.TempDir()
	configs := filepath.Join(root, "configs")
	require.NoError(t, os.MkdirAll(configs, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(configs, conf.DefaultsFileName), []byte("mysql:\n"+
		"  max_idle_conns: 50\n"+
		"  max_open_conns: 300\n"+
		"  max_idle_time: 300\n"+
		"  conn_max_lifetime: 1800\n"), 0o600))

	// 覆盖层不存在：仅基座生效。
	cfg, err := loadPoolConfig(root)
	require.NoError(t, err)
	require.Equal(t, 50, cfg.MaxIdleConns)
	require.Equal(t, 300, cfg.MaxOpenConns)
	require.Equal(t, 300, cfg.MaxIdleTime)
	require.Equal(t, 1800, cfg.ConnMaxLifetime)

	// 覆盖层存在：稀疏合并（未覆盖键保持基座值）。
	require.NoError(t, os.WriteFile(filepath.Join(configs, "config.yaml"), []byte("mysql:\n  max_open_conns: 42\n"), 0o600))
	cfg, err = loadPoolConfig(root)
	require.NoError(t, err)
	require.Equal(t, 42, cfg.MaxOpenConns)
	require.Equal(t, 50, cfg.MaxIdleConns)

	// 基座缺失：返回错误，由 NewDB 的零值回退兜底。
	_, err = loadPoolConfig(t.TempDir())
	require.Error(t, err)
}

// TestApplyPoolConfig 验证连接池参数实际应用到 sql.DB（sql.Open 惰性，
// 无需真实 MySQL）。
func TestApplyPoolConfig(t *testing.T) {
	db, err := sql.Open("mysql", "installer:pw@tcp(127.0.0.1:1)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local")
	require.NoError(t, err)
	defer db.Close()

	applyPoolConfig(db, conf.Database{MaxIdleConns: 50, MaxOpenConns: 300, MaxIdleTime: 300, ConnMaxLifetime: 1800})
	require.Equal(t, 300, db.Stats().MaxOpenConnections)

	// 零值回退默认。
	applyPoolConfig(db, conf.Database{})
	require.Equal(t, defaultMaxOpenConns, db.Stats().MaxOpenConnections)
}
