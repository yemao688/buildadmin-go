package main

import (
	appVersion "buildadmin-go/internal/pkg/version"
	"buildadmin-go/internal/conf"
	"testing"
	"time"
)

func TestVersionDefaultsToFrameworkVersion(t *testing.T) {
	if Version != appVersion.Framework {
		t.Fatalf("Version = %q, want %q", Version, appVersion.Framework)
	}
}

func TestVersionRequested(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-version"}, {"--conf", "configs/config.yaml", "--version"}} {
		if !versionRequested(args) {
			t.Fatalf("versionRequested(%v) = false", args)
		}
	}
	if versionRequested([]string{"migrate"}) {
		t.Fatal("versionRequested(migrate) = true")
	}
}

func TestMissingConfigMessage(t *testing.T) {
	if got := missingConfigMessage(true); got != "configs/config.yaml 不存在，setup 将以只读基座引导 CLI 安装" {
		t.Fatalf("setup missing config message = %q", got)
	}
	if got := missingConfigMessage(false); got != "configs/config.yaml 不存在，以只读基座启动安装向导，请访问 /install 完成安装（安装完成后会生成 configs/config.yaml）" {
		t.Fatalf("default missing config message = %q", got)
	}
}

func TestApplyTimeZone(t *testing.T) {
	original := time.Local
	t.Cleanup(func() { time.Local = original })

	if err := applyTimeZone("Asia/Shanghai"); err != nil {
		t.Fatalf("applyTimeZone(Asia/Shanghai) returned error: %v", err)
	}
	if got := time.Local.String(); got != "Asia/Shanghai" {
		t.Fatalf("time.Local = %q, want Asia/Shanghai", got)
	}
}

func TestApplyTimeZoneInvalidDoesNotChangeLocal(t *testing.T) {
	previous := time.Local
	original := time.FixedZone("existing", 1234)
	time.Local = original
	t.Cleanup(func() { time.Local = previous })

	if err := applyTimeZone("Not/A_Time_Zone"); err == nil {
		t.Fatal("applyTimeZone(invalid) returned nil error")
	}
	if time.Local != original {
		t.Fatalf("time.Local changed to %v, want %v", time.Local, original)
	}
}

func TestApplyTimeZoneEmptyUsesUTC(t *testing.T) {
	original := time.Local
	t.Cleanup(func() { time.Local = original })

	if err := applyTimeZone(""); err != nil {
		t.Fatalf("applyTimeZone(empty) returned error: %v", err)
	}
	if time.Local != time.UTC {
		t.Fatalf("time.Local = %v, want UTC", time.Local)
	}
}

func TestApplyAppRuntimeEnvironment(t *testing.T) {
	t.Setenv("APP_PORT", "9911")
	t.Setenv("APP_TIME_ZONE", "UTC")

	configuration := &conf.Configuration{}
	applyAppRuntimeEnvironment(configuration)
	if configuration.App.Port != "9911" || configuration.App.TimeZone != "UTC" {
		t.Fatalf("configuration app runtime values = %#v", configuration.App)
	}
}

func TestApplyAppRuntimeEnvironmentFallbacks(t *testing.T) {
	t.Setenv("APP_PORT", "")
	t.Setenv("APP_TIME_ZONE", "")

	configuration := &conf.Configuration{}
	applyAppRuntimeEnvironment(configuration)
	if configuration.App.Port != "9900" || configuration.App.TimeZone != "Asia/Shanghai" {
		t.Fatalf("configuration app fallback values = %#v", configuration.App)
	}
}
