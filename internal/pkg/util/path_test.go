package util

import (
	"os"
	"path/filepath"
	"testing"
)

// newRepoTree 构造含 go.mod 的最小仓库树，返回其根目录。
func newRepoTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// mkdirs 创建目录并返回其中最深的那个，用于拼二进制/源码路径。
func mkdirs(t *testing.T, root string, parts ...string) string {
	t.Helper()
	dir := filepath.Join(append([]string{root}, parts...)...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// findRootWithMarkers 认开发态与镜像态两种布局，且在没有 marker 时明确报告未命中。
func TestFindRootWithMarkers(t *testing.T) {
	root := newRepoTree(t)

	for name, start := range map[string]string{
		"二进制在根下一层": mkdirs(t, root, "tmp"),
		"二进制在根下两层": mkdirs(t, root, "runtime", "tmp"),
		"直接位于根目录":  root,
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := findRootWithMarkers(start)
			if !ok || got != root {
				t.Fatalf("findRootWithMarkers(%q) = (%q, %v), want (%q, true)", start, got, ok, root)
			}
		})
	}

	// 生产镜像只有 public/ 标记（/app/app -> /app）
	imageRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(imageRoot, "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := findRootWithMarkers(imageRoot); !ok || got != imageRoot {
		t.Fatalf("image layout: got (%q, %v), want (%q, true)", got, ok, imageRoot)
	}

	// 没有任何 marker 时必须报未命中，不能把猜到上一级当成根
	plain := t.TempDir()
	if got, ok := findRootWithMarkers(mkdirs(t, plain, "bin")); ok {
		t.Fatalf("plain layout: got (%q, true), want (\"\", false)", got)
	}
}

// resolveRootPath 的优先级：可执行文件 → 编译期源码 → 工作目录 → 历史回退。
func TestResolveRootPath(t *testing.T) {
	t.Run("可执行文件位于仓库内时优先", func(t *testing.T) {
		repo := newRepoTree(t)
		exe := filepath.Join(mkdirs(t, repo, "runtime", "tmp"), "server")
		otherRepo := newRepoTree(t)

		got := resolveRootPath(exe, filepath.Join(otherRepo, "internal", "pkg", "util", "path.go"), otherRepo)
		if got != repo {
			t.Fatalf("got %q, want %q（可执行文件所在仓库胜出）", got, repo)
		}
	})

	t.Run("二进制在 GOCACHE 内且环境变量未导出时回退编译期源码路径", func(t *testing.T) {
		// issue #18：`go env -w GOCACHE=…` 持久化后子进程读不到该变量，
		// go run 缓存命中时二进制直接位于 <GOCACHE>/<xx>/<hash>-d/ 下。
		cacheDir := t.TempDir()
		exe := filepath.Join(mkdirs(t, cacheDir, "e6", "e6c3e565fca189ef65080b7c7fba2140fd851489e43c0ba74df261f740a29c2f-d"), "server")
		repo := newRepoTree(t)

		got := resolveRootPath(exe, filepath.Join(repo, "internal", "pkg", "util", "path.go"), t.TempDir())
		if got != repo {
			t.Fatalf("got %q, want %q（缓存目录不能当根）", got, repo)
		}
	})

	t.Run("-trimpath 构建的编译期路径是相对路径时回退工作目录", func(t *testing.T) {
		repo := newRepoTree(t)
		exe := filepath.Join(t.TempDir(), "server")
		workDir := mkdirs(t, repo, "web")

		got := resolveRootPath(exe, filepath.Join("buildadmin-go", "internal", "pkg", "util", "path.go"), workDir)
		if got != repo {
			t.Fatalf("got %q, want %q（相对源码路径必须跳过）", got, repo)
		}
	})

	t.Run("三层都未命中时回退二进制上一级", func(t *testing.T) {
		plain := t.TempDir()
		exe := filepath.Join(mkdirs(t, plain, "bin"), "server")

		got := resolveRootPath(exe, filepath.Join("buildadmin-go", "internal", "pkg", "util", "path.go"), t.TempDir())
		if want := filepath.Dir(exe); got != want {
			t.Fatalf("got %q, want %q（历史回退）", got, want)
		}
	})
}

// 真实执行环境下 computeRootPath 必须指向仓库根：任何一层猜错都会让 CLI
// 把 configs/config.yaml 拼到不存在的目录并误报"未安装"（issue #18）。
func TestComputeRootPathFindsRepoRoot(t *testing.T) {
	root := computeRootPath()
	if !PathExists(filepath.Join(root, "go.mod")) {
		t.Fatalf("computeRootPath() = %q，不含 go.mod", root)
	}
	if !PathExists(filepath.Join(root, "internal", "pkg", "util", "path.go")) {
		t.Fatalf("computeRootPath() = %q，不含本包源码", root)
	}
}
