package commands

import (
	"os"
	"path/filepath"
	"testing"
)

// 目录参数必须展开成按文件名排序的 spec 文件列表，approveCommand 才能直接执行。
func TestExpandSpecPathsExpandsDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"b.yaml", "a.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("name: x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "schema.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, err := expandSpecPaths([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || filepath.Base(paths[0]) != "a.yaml" || filepath.Base(paths[1]) != "b.yaml" {
		t.Fatalf("unexpected expansion: %v", paths)
	}
}

func TestExpandSpecPathsRejectsEmptyDirectory(t *testing.T) {
	t.Parallel()
	if _, err := expandSpecPaths([]string{t.TempDir()}); err == nil {
		t.Fatal("empty spec directory must fail")
	}
}
