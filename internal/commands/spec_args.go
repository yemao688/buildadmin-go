package commands

import (
	helper "buildadmin-go/internal/crudgen"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		// 展开结果按仓库相对路径输出：approveCommand 要能直接复制到仓库根执行。
		for _, match := range matches {
			paths = append(paths, relativeToRoot(match))
		}
	}
	return paths, nil
}
