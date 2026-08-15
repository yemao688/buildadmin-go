package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buildadmin-go/internal/pkg/util"
)

func TestFrameworkVersionFileMatchesConstant(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(util.RootPath(), "VERSION_FRAMEWORK"))
	if err != nil {
		t.Fatalf("read VERSION_FRAMEWORK: %v", err)
	}

	if got := strings.TrimSpace(string(data)); got != Framework {
		t.Fatalf("VERSION_FRAMEWORK = %q, want %q", got, Framework)
	}
}

// TestChangelogCoversCurrentVersion 保证发版纪律：CHANGELOG.md 必须包含当前
// 框架版本的条目，防止版本号提升但变更记录缺失。
func TestChangelogCoversCurrentVersion(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(util.RootPath(), "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("read CHANGELOG.md: %v", err)
	}
	want := "## v" + Framework
	if !strings.Contains(string(data), want) {
		t.Fatalf("CHANGELOG.md missing entry %q for framework version %s", want, Framework)
	}
}

// TestDisplayVersion 验证展示版本语义：未注入业务版本（dev）时回退框架
// 版本；注入业务版本后以业务版本为主、框架版本为辅（业务镜像 ldflags
// 注入，线上 banner/--version 显示业务版本号）。
func TestDisplayVersion(t *testing.T) {
	original := Business
	t.Cleanup(func() { Business = original })

	// 未注入（本地 go run / 框架开发）：仅框架版本
	Business = "dev"
	if got := Display(); got != Framework {
		t.Fatalf("Display() without business version = %q, want %q", got, Framework)
	}
	// 空值同样回退
	Business = ""
	if got := Display(); got != Framework {
		t.Fatalf("Display() with empty business = %q, want %q", got, Framework)
	}
	// dev 前缀（无业务 VERSION 文件时 Makefile 默认 VERSION=dev，Dockerfile
	// 注入 "dev-<gitsha>-<ts>"）：仍按未注入处理，回退框架版本
	Business = "dev-abc1234-20260101T000000Z"
	if got := Display(); got != Framework {
		t.Fatalf("Display() with dev-prefixed business = %q, want %q", got, Framework)
	}
	// 注入业务版本（模拟 ldflags -X version.Business=1.0.0-gitabc-ts）：
	// 业务版本主显 + 框架版本标注
	Business = "1.0.0-gitabc-ts"
	want := "1.0.0-gitabc-ts (framework " + Framework + ")"
	if got := Display(); got != want {
		t.Fatalf("Display() with business version = %q, want %q", got, want)
	}
}
