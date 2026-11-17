package crudgen

import (
	"buildadmin-go/internal/pkg/data_scope"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ownerSpecDeclaration 只读取判定归属所需的两个键：reassignable 与 inheritFrom。
// 刻意不跑完整 LoadSpec：启动期登记不应因某个 spec 的其它字段有问题而整体失败，
// 但 YAML 本身解析失败仍然报错（坏文件不能静默变成"无需登记"）。
type ownerSpecDeclaration struct {
	Name      string `yaml:"name"`
	DataScope struct {
		Reassignable bool `yaml:"reassignable"`
		InheritFrom  *struct {
			Table string `yaml:"table"`
		} `yaml:"inheritFrom"`
	} `yaml:"dataScope"`
}

// CollectOwnerTables 扫描 spec 目录，返回声明了归属语义（reassignable 或
// inheritFrom）的逻辑表名，按目录顺序去重。它是删除管理员时转移清单的事实源，
// 与手写登记互补：手写登记只保留给没有 spec 的非 CRUD 表。
func CollectOwnerTables(specDir string) ([]string, error) {
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return nil, fmt.Errorf("read spec directory %s: %w", specDir, err)
	}
	tables := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(specDir, entry.Name())) //nolint:gosec // G304: 文件名来自 specDir 的 ReadDir，路径固定在仓库/镜像内
		if err != nil {
			return nil, fmt.Errorf("read spec %s: %w", entry.Name(), err)
		}
		var declaration ownerSpecDeclaration
		if err := yaml.Unmarshal(content, &declaration); err != nil {
			return nil, fmt.Errorf("parse spec %s: %w", entry.Name(), err)
		}
		if !declaration.DataScope.Reassignable && declaration.DataScope.InheritFrom == nil {
			continue
		}
		name := strings.TrimSpace(declaration.Name)
		if name == "" {
			return nil, fmt.Errorf("spec %s declares owner semantics but has no name", entry.Name())
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		tables = append(tables, name)
	}
	return tables, nil
}

// RegisterSpecOwnerTables 把 spec 声明的归属表登记进删除管理员的转移清单。
// 重复登记无害，调用方可以在每次启动时执行。
func RegisterSpecOwnerTables(specDir string) (int, error) {
	tables, err := CollectOwnerTables(specDir)
	if err != nil {
		return 0, err
	}
	if len(tables) == 0 {
		return 0, nil
	}
	if err := data_scope.RegisterAdminOwnerTables(tables...); err != nil {
		return 0, fmt.Errorf("register spec owner tables: %w", err)
	}
	return len(tables), nil
}
