package core

import (
	"fmt"
	"testing"
	"time"

	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// 该用例会建/删整批核心表，走 MySQL 门禁；未配置 mysql_test 时明确 SKIP。
func TestEnsureMissingCoreTablesCreatesOnlyMissingTablesMySQL(t *testing.T) {
	t.Parallel()
	db, baseCfg := testutil.OpenMySQL(t)
	cfg := *baseCfg
	cfg.Database.Prefix = fmt.Sprintf("ensure_core_%d_", time.Now().UnixNano())
	db = db.Session(&gorm.Session{NewDB: true})
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	t.Cleanup(func() {
		for _, logical := range append(CoreLogicalNames(), "migrations", "migrations_framework", "migrations_business") {
			db.Exec("DROP TABLE IF EXISTS " + QuoteIdentifier(cfg.Database.Prefix+logical))
		}
	})

	// 预置一张故意不完整的已有表：补表逻辑不得改动它。
	adminTable := TableName(&cfg, "admin")
	require.NoError(t, db.Exec("CREATE TABLE "+QuoteIdentifier(adminTable)+" (id INT PRIMARY KEY, legacy_marker INT)").Error)

	created, err := EnsureMissingCoreTables(db, cfg.Database.Prefix)
	require.NoError(t, err)
	require.True(t, created)
	for _, logical := range CoreLogicalNames() {
		require.True(t, db.Migrator().HasTable(TableName(&cfg, logical)), "core table %s is missing", logical)
	}
	require.True(t, ColumnExists(db, adminTable, "legacy_marker"))
	require.False(t, ColumnExists(db, adminTable, "username"))

	// 第二次调用不再补建，已存在表保持原形状。
	created, err = EnsureMissingCoreTables(db, cfg.Database.Prefix)
	require.NoError(t, err)
	require.False(t, created)
	require.True(t, ColumnExists(db, adminTable, "legacy_marker"))
}
