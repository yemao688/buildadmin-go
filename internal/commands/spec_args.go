package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// expandSpecPaths 把目录参数展开为按文件名排序的 spec 文件列表：crud:apply 与
// approveCommand 共用同一条展开规则，保证 JSON/HTML 里给出的审批命令可以直接
// 复制执行（此前目录会被当成 spec 文件传给 LoadSpec 而报 "is a directory"）。
func expandSpecPaths(args []string) ([]string, error) {
	if len(args) == 0 {
		dir := helper.DefaultSpecDir()
		if dir == "" {
			return nil, fmt.Errorf("no spec directory crud_specs found; pass spec files explicitly: crud:apply <spec.yaml...>")
		}
		args = []string{dir}
	}
	paths := make([]string, 0, len(args))
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("stat spec %s: %w", arg, err)
		}
		if !info.IsDir() {
			paths = append(paths, arg)
			continue
		}
		matches, err := filepath.Glob(filepath.Join(arg, "*.yaml"))
		if err != nil {
			return nil, fmt.Errorf("scan spec directory %s: %w", arg, err)
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no spec files found in %s", arg)
		}
		sort.Strings(matches)
		paths = append(paths, matches...)
	}
	// 执行用绝对路径（允许从任意 cwd 运行），展示/approveCommand 再压成仓库相对路径。
	unique := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve spec %s: %w", path, err)
		}
		if _, exists := seen[absolute]; exists {
			continue
		}
		seen[absolute] = struct{}{}
		unique = append(unique, absolute)
	}
	return unique, nil
}

// relativeSpecRef 把展开后的绝对 spec 文件压成仓库相对路径列表，供
// approveCommand 与审批页展示（可从仓库根直接复制执行）。
func relativeSpecRef(paths []string) string {
	display := make([]string, 0, len(paths))
	for _, path := range paths {
		display = append(display, relativeToRoot(path))
	}
	return strings.Join(display, " ")
}
