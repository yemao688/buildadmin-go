package db

import (
	"strings"
	"testing"
	"time"

	"buildadmin-go/internal/conf"
)

// TestBuildDSNIncludesTimeouts 验证 DSN 拼接包含连接/读写超时参数，防止
// 卡死连接无限阻塞（go-sql-driver 读写超时默认 0 = 永不超时）。
func TestBuildDSNIncludesTimeouts(t *testing.T) {
	cfg := &conf.Database{
		Host: "127.0.0.1", Port: 3306, Database: "buildadmin",
		UserName: "root", Password: "secret", Charset: "utf8mb4",
	}
	dsn := buildDSN(cfg)
	for _, want := range []string{
		"root:secret@tcp(127.0.0.1:3306)/buildadmin",
		"charset=utf8mb4",
		"parseTime=True",
		"loc=Local",
		"timeout=5s",
		"readTimeout=5s",
		"writeTimeout=5s",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("dsn %q missing %q", dsn, want)
		}
	}
}

// TestConnMaxLifetimeFallback 验证未配置（0）时回退 1800s，配置生效时按秒取值。
func TestConnMaxLifetimeFallback(t *testing.T) {
	cfg := &conf.Database{}
	if got := connMaxLifetime(cfg); got != 1800*time.Second {
		t.Errorf("unset conn_max_lifetime = %v, want 1800s", got)
	}
	cfg.ConnMaxLifetime = 3600
	if got := connMaxLifetime(cfg); got != 3600*time.Second {
		t.Errorf("conn_max_lifetime = %v, want 3600s", got)
	}
}

// TestConnMaxIdleTimeFallback 验证未配置（0）时回退 300s，配置生效时按秒取值。
func TestConnMaxIdleTimeFallback(t *testing.T) {
	cfg := &conf.Database{}
	if got := connMaxIdleTime(cfg); got != 300*time.Second {
		t.Errorf("unset max_idle_time = %v, want 300s", got)
	}
	cfg.MaxIdleTime = 600
	if got := connMaxIdleTime(cfg); got != 600*time.Second {
		t.Errorf("max_idle_time = %v, want 600s", got)
	}
}
