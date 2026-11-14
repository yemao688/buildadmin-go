package data_scope

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // 测试向进程级属主表注册表写入且无清理，并行会污染同包其它用例
func TestRegisterAdminOwnerTables(t *testing.T) {
	require.NoError(t, RegisterAdminOwnerTables("owner_registry_a", "owner_registry_a", "owner_registry_b"))
	require.NoError(t, RegisterAdminOwnerTables("owner_registry_b"))

	tables := AdminOwnerTables()
	require.Equal(t, 1, countOwnerTable(tables, "owner_registry_a"))
	require.Equal(t, 1, countOwnerTable(tables, "owner_registry_b"))

	tables[0] = "mutated"
	require.NotContains(t, AdminOwnerTables(), "mutated")
}

//nolint:paralleltest // 断言全局属主表注册表在调用前后不变，需与写入该注册表的用例串行
func TestRegisterAdminOwnerTablesRejectsInvalidBatch(t *testing.T) {
	before := AdminOwnerTables()
	require.Error(t, RegisterAdminOwnerTables("owner_valid_batch", "owner-bad-name"))
	require.Equal(t, before, AdminOwnerTables())
}

func countOwnerTable(tables []string, target string) int {
	count := 0
	for _, table := range tables {
		if table == target {
			count++
		}
	}
	return count
}
