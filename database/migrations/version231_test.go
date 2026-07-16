package migrations

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openVersion231TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestVersion231AddsCommitFlagWithExistingRows(t *testing.T) {
	db := openVersion231TestDB(t)
	prefix := fmt.Sprintf("v231_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	tables := []string{tableName(cfg, "security_data_recycle_log"), tableName(cfg, "security_sensitive_data_log")}
	for _, table := range tables {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
		require.NoError(t, db.Exec("CREATE TABLE "+q(table)+" (id INT PRIMARY KEY)").Error)
		require.NoError(t, db.Exec("INSERT INTO "+q(table)+" VALUES (1)").Error)
	}
	require.NoError(t, version231(db, cfg))
	for _, table := range tables {
		var committed int
		require.NoError(t, db.Raw("SELECT is_committed FROM "+q(table)+" WHERE id=1").Scan(&committed).Error)
		require.Equal(t, 0, committed)
	}
	require.NoError(t, version231(db, cfg))
}

func TestVersion231FreshEmptyAndPrefix(t *testing.T) {
	db := openVersion231TestDB(t)
	prefix := fmt.Sprintf("v231empty_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	table := tableName(cfg, "security_data_recycle_log")
	db.Exec("DROP TABLE IF EXISTS " + q(table))
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	require.NoError(t, db.Exec("CREATE TABLE "+q(table)+" (id INT PRIMARY KEY)").Error)
	require.NoError(t, version231(db, cfg))
	require.True(t, columnExists(db, table, "is_committed"))
}

func TestVersion231RejectsInvalidExistingSchema(t *testing.T) {
	db := openVersion231TestDB(t)
	prefix := fmt.Sprintf("v231bad_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	table := tableName(cfg, "security_sensitive_data_log")
	db.Exec("DROP TABLE IF EXISTS " + q(table))
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	require.NoError(t, db.Exec("CREATE TABLE "+q(table)+" (id INT PRIMARY KEY, is_committed TINYINT NULL DEFAULT 1)").Error)
	err := version231(db, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid schema")
}
