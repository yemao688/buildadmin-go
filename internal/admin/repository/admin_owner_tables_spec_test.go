package repository

import (
	"os"
	"path/filepath"
	"testing"

	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/util"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// crud_specs 的 dataScope.reassignable / dataScope.inheritFrom 是归属关系的事实
// 源，data_scope.RegisterAdminOwnerTables 是删除管理员时的转移清单；两者是
// 手工维护的第二份清单，漏登记会让删除后的归属数据静默变成孤儿。本测试只用
// yaml.v3 读取顶层 name 与 dataScope，不依赖数据库、生成器或 crud_log。
func TestAdminOwnerTablesCoverCrudSpecs(t *testing.T) {
	t.Parallel()
	specDir := filepath.Join(util.RootPath(), "crud_specs")
	entries, err := os.ReadDir(specDir)
	require.NoError(t, err)

	registered := make(map[string]struct{}, len(data_scope.AdminOwnerTables()))
	for _, table := range data_scope.AdminOwnerTables() {
		registered[table] = struct{}{}
	}

	checked := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		content, err := os.ReadFile(filepath.Join(specDir, entry.Name())) //nolint:gosec // specDir 固定为仓库 crud_specs 目录，文件名来自该目录 ReadDir
		require.NoError(t, err)

		var spec struct {
			Name      string `yaml:"name"`
			DataScope struct {
				Reassignable bool `yaml:"reassignable"`
				InheritFrom  *struct {
					Table string `yaml:"table"`
				} `yaml:"inheritFrom"`
			} `yaml:"dataScope"`
		}
		if err := yaml.Unmarshal(content, &spec); err != nil {
			// crud_specs 由生成器统一维护，格式只有一种；解析失败是真正的坏
			// spec，直接失败，避免静默跳过把漏登记掩盖成"无需登记"。
			t.Fatalf("parse %s: %v", entry.Name(), err)
		}
		if !spec.DataScope.Reassignable && spec.DataScope.InheritFrom == nil {
			continue
		}
		require.NotEmpty(t, spec.Name, "%s must declare name", entry.Name())
		require.Contains(t, registered, spec.Name,
			"%s declares owner semantics (reassignable/inheritFrom) but %q is missing from data_scope.RegisterAdminOwnerTables",
			entry.Name(), spec.Name)
		checked++
	}
	require.NotZero(t, checked, "no owner-declaring crud_specs found; the consistency test would be vacuous")
}
