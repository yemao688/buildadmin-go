package util

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	rootPathOnce sync.Once
	rootPathVal  string
)

// RootPath 获取项目根目录绝对路径（首次调用计算后缓存复用；
// os.Executable/UserHomeDir/逐级 Stat 的开销只在首次发生）
func RootPath() string {
	rootPathOnce.Do(func() {
		rootPathVal = computeRootPath()
	})
	return rootPathVal
}

func computeRootPath() string {
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	// go run 会将编译产物放在临时目录或 build cache 中，
	// 此时 os.Executable() 返回的路径不在项目目录下，需要用编译期路径回退。
	// 回退要求编译期路径是绝对路径：生产镜像以 -trimpath 构建，runtime.Caller
	// 只返回相对路径（buildadmin-go/internal/...），拿它推断根目录会得到垃圾值。
	if isTempPath(exePath) {
		_, filename, _, ok := runtime.Caller(0)
		if ok && filepath.IsAbs(filename) {
			return findProjectRoot(filepath.Dir(filename))
		}
	}

	return findProjectRoot(filepath.Dir(exePath))
}

// rootMarkers 标识项目根目录的特征文件/目录：
// 开发环境有 go.mod 与 configs/config.defaults.yaml；生产镜像只有 public/。
var rootMarkers = []string{"go.mod", "configs/config.defaults.yaml", "public"}

// findProjectRoot 从 startDir 逐级向上查找包含任一 rootMarker 的目录，
// 使二进制位于根下任意深度（tmp/、runtime/tmp/ 等）都能正确定位根目录；
// 找不到时回退到 startDir 的上一级（兼容历史"二进制位于根下一层"的假设）。
func findProjectRoot(startDir string) string {
	dir := startDir
	for {
		for _, marker := range rootMarkers {
			if PathExists(filepath.Join(dir, marker)) {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Dir(startDir)
		}
		dir = parent
	}
}

// isTempPath 判断路径是否位于系统临时目录或 Go build cache 中
func isTempPath(path string) bool {
	if strings.Contains(path, os.TempDir()) {
		return true
	}
	for _, buildCache := range goBuildCacheDirs() {
		if strings.Contains(path, buildCache) {
			return true
		}
	}
	return false
}

// goBuildCacheDirs 返回 go 工具链可能存放 go run 编译产物的缓存目录。
// GOCACHE 优先：用户可把它指到任意磁盘/CI 目录，此时产物不在默认缓存路径下；
// 未设置时回退 os.UserCacheDir()/go-build，即 go 自身的跨平台默认值
// （macOS ~/Library/Caches、Linux XDG_CACHE_HOME、Windows LocalAppData）。
func goBuildCacheDirs() []string {
	dirs := make([]string, 0, 2)
	if gocache := os.Getenv("GOCACHE"); gocache != "" {
		dirs = append(dirs, gocache)
	}
	if userCacheDir, err := os.UserCacheDir(); err == nil {
		dirs = append(dirs, filepath.Join(userCacheDir, "go-build"))
	}
	return dirs
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}
