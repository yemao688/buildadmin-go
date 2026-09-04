package repository

import (
	"fmt"
	"testing"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/testutil"
	"github.com/stretchr/testify/require"
)

func TestStripTablePrefix(t *testing.T) {
	cases := []struct {
		table  string
		prefix string
		want   string
	}{
		{"ba_admin", "ba_", "admin"},
		{"ba_test", "ba_", "test"},
		{"BA_admin", "ba_", "admin"}, // 大小写不敏感，对齐上游 /i
		{"other_table", "ba_", "other_table"},
		{"admin", "ba_", "admin"},
		{"ba_admin", "", "ba_admin"},
		{"ba_", "ba_", ""},
	}
	for _, c := range cases {
		if got := StripTablePrefix(c.table, c.prefix); got != c.want {
			t.Errorf("StripTablePrefix(%q, %q) = %q, want %q", c.table, c.prefix, got, c.want)
		}
	}
}

// TestGetColumnsFallsBackToConnectionDatabase 回归：cfg.Database 为空（如
// freshMigrationDatabase 仅设 Prefix）时，GetColumns 回退 DATABASE() 而非
// 空 schema 参数静默返回 0 列（曾导致 EnsureSpecTable 二次物化误判
// primary key drift）。
func TestGetColumnsFallsBackToConnectionDatabase(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	prefix := fmt.Sprintf("tbl_repo_%d_", time.Now().UnixNano())
	tableName := prefix + "member"

	if err := db.Exec("DROP TABLE IF EXISTS `" + tableName + "`").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS `" + tableName + "`")
	})
	if err := db.Exec("CREATE TABLE `" + tableName + "` (`id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, `name` VARCHAR(191) NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}

	// 复现调查场景：cfg 仅含 Prefix、不设 Database（下游 EnsureSpecTable 路径）
	repo := NewTableRepository(&conf.Configuration{Database: conf.Database{Prefix: prefix}}, db)
	columns, err := repo.GetColumns("member")
	require.NoError(t, err)
	require.Len(t, columns, 2, "cfg.Database 为空时 GetColumns 应回退当前连接库，返回真实列而非 0 列")

	// 参照组：显式 Database 的 cfg 行为一致（仅 Database 字段不同，前缀相同）
	repoWithDB := NewTableRepository(&conf.Configuration{Database: conf.Database{Prefix: prefix, Database: cfg.Database.Database}}, db)
	columnsWithDB, err := repoWithDB.GetColumns("member")
	require.NoError(t, err)
	require.Len(t, columnsWithDB, 2)
}
