package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsTempPath(t *testing.T) {
	sep := string(filepath.Separator)
	// HOME/XDG_CACHE_HOME 指到临时目录之外，让 UserCacheDir 的默认缓存路径可预期
	// （darwin 读 HOME/Library/Caches，其他平台读 XDG_CACHE_HOME 或 HOME/.cache）。
	homeDir := filepath.Join(sep, "issue18-home")
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(homeDir, "xdg-cache"))

	customCache := filepath.Join(sep, "issue18-custom-gocache")
	defaultCache := filepath.Join(sep, "issue18-unknown-cache", "go-build")
	if userCacheDir, err := os.UserCacheDir(); err == nil {
		defaultCache = filepath.Join(userCacheDir, "go-build")
	}

	tests := []struct {
		name    string
		gocache string
		path    string
		want    bool
	}{
		{
			name:    "GOCACHE 下的 go run 产物",
			gocache: customCache,
			path:    filepath.Join(customCache, "3f", "exe"),
			want:    true,
		},
		{
			name:    "默认缓存下的 go run 产物",
			gocache: "",
			path:    filepath.Join(defaultCache, "ab", "exe"),
			want:    true,
		},
		{
			name:    "系统临时目录",
			gocache: "",
			path:    filepath.Join(os.TempDir(), "go-build-xx", "exe"),
			want:    true,
		},
		{
			name:    "普通源码目录不误判",
			gocache: customCache,
			path:    filepath.Join(sep, "issue18-project", "runtime", "tmp", "server"),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOCACHE", tt.gocache)
			if got := isTempPath(tt.path); got != tt.want {
				t.Errorf("isTempPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFindProjectRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 二进制位于根下一层（历史 air 布局 tmp/main）
	tmpDir := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findProjectRoot(tmpDir); got != root {
		t.Errorf("tmp layout: got %q, want %q", got, root)
	}

	// 二进制位于根下两层（现 air 布局 runtime/tmp/main）
	nested := filepath.Join(root, "runtime", "tmp")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findProjectRoot(nested); got != root {
		t.Errorf("runtime/tmp layout: got %q, want %q", got, root)
	}

	// 生产镜像布局：只有 public/ 标记（/app/app -> /app）
	imgRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(imgRoot, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findProjectRoot(imgRoot); got != imgRoot {
		t.Errorf("image layout: got %q, want %q", got, imgRoot)
	}

	// 无标记时回退到上一级（兼容历史假设）
	plain := t.TempDir()
	binDir := filepath.Join(plain, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findProjectRoot(binDir); got != plain {
		t.Errorf("fallback: got %q, want %q", got, plain)
	}
}
