package commands

import (
	"testing"

	"buildadmin-go/internal/conf"
)

// 解析顺序固定为环境变量 > 配置 > apply，非法值按 check 处理（不可放大为 apply）。
func TestSchemaModeFromConfig(t *testing.T) {
	tests := []struct {
		name   string
		env    string
		config string
		want   string
	}{
		{name: "配置为空默认 apply", env: "", config: "", want: schemaModeApply},
		{name: "配置 off", env: "", config: "off", want: schemaModeOff},
		{name: "配置 check", env: "", config: "check", want: schemaModeCheck},
		{name: "配置 apply", env: "", config: "apply", want: schemaModeApply},
		{name: "配置非法值回退 check", env: "", config: "bogus", want: schemaModeCheck},
		{name: "环境变量覆盖配置", env: "check", config: "off", want: schemaModeCheck},
		{name: "环境变量去空白并忽略大小写", env: " APPLY ", config: "off", want: schemaModeApply},
		{name: "环境变量非法值回退 check", env: "bogus", config: "check", want: schemaModeCheck},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("APP_AUTO_MIGRATE", tt.env)
			cfg := &conf.Configuration{}
			cfg.App.AutoMigrate = tt.config
			if got := schemaModeFromConfig(cfg); got != tt.want {
				t.Fatalf("schemaModeFromConfig() = %q, want %q", got, tt.want)
			}
		})
	}
}
