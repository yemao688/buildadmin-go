package installer

import (
	"os"
	"path/filepath"
	"testing"

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

	require.NoError(t, WriteCompletionLock(root))
	content, err := os.ReadFile(lockPath)
	require.NoError(t, err)
	require.Equal(t, []byte(InstallationCompletionMark), content)
}

func TestWriteBaseConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
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
	err := WriteBaseConfig(filepath.Join(t.TempDir(), "config.yaml"), Database{Hostport: "not-a-port"}, "token")
	require.Error(t, err)
}

func TestGenerateTokenKey(t *testing.T) {
	key := GenerateTokenKey()
	require.Len(t, key, 32)
	for _, char := range key {
		require.Contains(t, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", string(char))
	}
}
