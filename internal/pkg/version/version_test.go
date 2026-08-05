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
