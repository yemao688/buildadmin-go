package core

import (
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// coreEnsureProbe 只用于验证补表逻辑的方言无关行为；tag 必须是 sqlite 可解析的。
type coreEnsureProbe struct {
	ID     int64  `gorm:"column:id;primaryKey;autoIncrement"`
	Name   string `gorm:"column:name;type:varchar(20);not null;default:''"`
	Legacy int    `gorm:"column:legacy_marker"`
}

// probeSQLiteSeq 与 pid 组合出进程内唯一的内存库名：并行子测试与 -count=N
// 可能在同一时钟刻度内重复调用，时间戳不足以保证唯一，重名会让两个测试共享
// 同一个 cache=shared 内存库并互相锁表。
var probeSQLiteSeq atomic.Uint64

func openProbeSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:core-ensure-%d-%d?mode=memory&cache=shared", os.Getpid(), probeSQLiteSeq.Add(1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: "probe_"}
	return db
}

// hasColumn 用 ColumnTypes 断言列存在，避免 Migrator().HasColumn(string, ...)
// 在部分方言下依赖 statement schema 的行为差异。
func hasColumn(t *testing.T, db *gorm.DB, table, column string) bool {
	t.Helper()
	columnTypes, err := db.Migrator().ColumnTypes(table)
	require.NoError(t, err)
	for _, columnType := range columnTypes {
		if columnType.Name() == column {
			return true
		}
	}
	return false
}

func TestEnsureMissingCoreTableCreatesOnlyAbsentTable(t *testing.T) {
	t.Parallel()
	table := CoreTable{
		LogicalName: "core_ensure_probe",
		Comment:     "探针表",
		NewModel:    func() any { return &coreEnsureProbe{} },
	}
	fullTable := "probe_core_ensure_probe"

	t.Run("missing table is created", func(t *testing.T) {
		t.Parallel()
		db := openProbeSQLite(t)
		created, err := ensureMissingCoreTable(db, "probe_", table)
		require.NoError(t, err)
		require.True(t, created)
		require.True(t, db.Migrator().HasTable(&coreEnsureProbe{}))
		require.True(t, hasColumn(t, db, fullTable, "name"))

		created, err = ensureMissingCoreTable(db, "probe_", table)
		require.NoError(t, err)
		require.False(t, created)
	})

	t.Run("existing table is left untouched", func(t *testing.T) {
		t.Parallel()
		db := openProbeSQLite(t)
		require.NoError(t, db.Exec("CREATE TABLE `"+fullTable+"` (id INTEGER PRIMARY KEY, legacy_marker INTEGER)").Error)

		created, err := ensureMissingCoreTable(db, "probe_", table)
		require.NoError(t, err)
		require.False(t, created)
		require.True(t, hasColumn(t, db, fullTable, "legacy_marker"))
		require.False(t, hasColumn(t, db, fullTable, "name"))
	})
}
