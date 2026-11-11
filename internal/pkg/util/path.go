package util

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// RootPath 返回项目根目录绝对路径；结果在首次调用后缓存（os.Executable 与
// 逐级 Stat 的开销只发生一次）。
func RootPath() string {
	return rootPath()
}

var rootPath = sync.OnceValue(computeRootPath)

func computeRootPath() string {
	exePath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	_, filename, _, _ := runtime.Caller(0)
	workDir, err := os.Getwd()
	if err != nil {
		workDir = ""
	}
	return resolveRootPath(exePath, filename, workDir)
}

// resolveRootPath 定位项目根目录，逐层要求真的命中 rootMarkers：
//
//  1. 可执行文件所在目录——生产镜像与常规安装（二进制就在根下或其子目录）；
//  2. 编译期源码路径——go run 冷编译时产物在 $TMPDIR，二进制目录没有 marker；
//     -trimpath 构建返回相对路径，此时跳过；
//  3. 当前工作目录——go run 缓存命中时二进制直接从 $GOCACHE/<xx>/<hash>-d/
//     执行，而 GOCACHE 可经 `go env -w` 持久化、子进程读不到该环境变量，
//     前两层可能都落空，退回用户执行命令的目录；
//  4. 都未命中时保留历史回退：二进制所在目录的上一级。
//
// 每层都必须命中 marker 才算数："未命中就回退上一级"的猜测语义会把错的目录
// 当成根（issue #18 的错误根正是这么来的）。
func resolveRootPath(exePath, callerFile, workDir string) string {
	if root, ok := findRootWithMarkers(filepath.Dir(exePath)); ok {
		return root
	}
	if callerFile != "" && filepath.IsAbs(callerFile) {
		if root, ok := findRootWithMarkers(filepath.Dir(callerFile)); ok {
			return root
		}
	}
	if workDir != "" {
		if root, ok := findRootWithMarkers(workDir); ok {
			return root
		}
	}
	return filepath.Dir(exePath)
}

// rootMarkers 标识项目根目录的特征文件/目录：
// 开发环境有 go.mod 与 configs/config.defaults.yaml；生产镜像只有 public/。
var rootMarkers = []string{"go.mod", "configs/config.defaults.yaml", "public"}

// findRootWithMarkers 从 startDir 逐级向上查找包含任一 rootMarker 的目录；
// 一直找到文件系统根仍未命中时返回 ("", false)。
func findRootWithMarkers(startDir string) (string, bool) {
	dir := startDir
	for {
		for _, marker := range rootMarkers {
			if PathExists(filepath.Join(dir, marker)) {
				return dir, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
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
