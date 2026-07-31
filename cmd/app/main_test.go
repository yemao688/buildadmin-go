package main

import (
	appVersion "go-build-admin/app/pkg/version"
	"go-build-admin/conf"
	"testing"
	"time"
)

func TestVersionDefaultsToFrameworkVersion(t *testing.T) {
	if Version != appVersion.Framework {
		t.Fatalf("Version = %q, want %q", Version, appVersion.Framework)
	}
}

func TestVersionRequested(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-version"}, {"--conf", "config.yaml", "--version"}} {
		if !versionRequested(args) {
			t.Fatalf("versionRequested(%v) = false", args)
		}
	}
	if versionRequested([]string{"migrate"}) {
		t.Fatal("versionRequested(migrate) = true")
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
	if configuration.App.Port != "9989" || configuration.App.TimeZone != "Asia/Shanghai" {
		t.Fatalf("configuration app fallback values = %#v", configuration.App)
	}
}
