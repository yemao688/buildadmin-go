package commands

import (
	"buildadmin-go/internal/conf"
	appVersion "buildadmin-go/internal/pkg/version"
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestVersionDefaultsToFrameworkVersion(t *testing.T) {
	// 未注入业务版本（Business == "dev"）时 Display() 回退框架版本，
	// 线上业务镜像经 ldflags 注入后 Version 即为业务版本 + 框架标注。
	if Version != appVersion.Display() {
		t.Fatalf("Version = %q, want %q", Version, appVersion.Display())
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
	if got := missingConfigMessage(false); got != "configs/config.yaml 不存在，请先执行 setup 完成安装（安装完成后会自动生成该配置文件）" {
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

func TestRootHelpListsAllCommands(t *testing.T) {
	var output bytes.Buffer
	root := newRootCommand(nil, nil)
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(--help) error = %v", err)
	}
	for _, want := range []string{"server", "example", "migrate", "setup", "crud:generate", "crud:delete", "crud:apply", "crud:validate"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("help output missing %q:\n%s", want, output.String())
		}
	}
}

func TestCrudValidateHelp(t *testing.T) {
	var output bytes.Buffer
	root := newRootCommand(nil, nil)
	root.SetOut(&output)
	root.SetErr(&output)
	root.SetArgs([]string{"crud:validate", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute(crud:validate --help) error = %v", err)
	}
	if !strings.Contains(output.String(), "crud:validate") {
		t.Fatalf("help output = %q", output.String())
	}
}

func TestExecuteVersionShortCircuit(t *testing.T) {
	original := os.Args
	t.Cleanup(func() { os.Args = original })
	os.Args = []string{"app", "--version"}
	if err := Execute(nil, nil); err != nil {
		t.Fatalf("Execute(--version) error = %v", err)
	}
}
