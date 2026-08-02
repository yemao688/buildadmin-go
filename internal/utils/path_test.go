package utils

import (
	"os"
	"path/filepath"
	"testing"
)

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
