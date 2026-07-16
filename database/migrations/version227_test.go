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

func openMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestVersion227RepairsZeroNullAndDanglingOwners(t *testing.T) {
	db := openMigrationTestDB(t)
	prefix := fmt.Sprintf("v227_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	admin := tableName(cfg, "admin")
	log := tableName(cfg, "admin_log")
	for _, table := range []string{admin, log} {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(admin)+" (id INT PRIMARY KEY, parent_id INT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(admin)+" VALUES (1,NULL),(2,1)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(log)+" (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(log)+" VALUES (1,0),(2,999),(3,2)").Error)
	require.NoError(t, version227(db, cfg))
	var invalid int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM "+q(log)+" l LEFT JOIN "+q(admin)+" a ON a.id=l.admin_id WHERE a.id IS NULL OR l.admin_id=0").Scan(&invalid).Error)
	require.Zero(t, invalid)
	var rootCount int64
	require.NoError(t, db.Table(log).Where("admin_id=1").Count(&rootCount).Error)
	require.Equal(t, int64(2), rootCount)
	require.NoError(t, version227(db, cfg))
}

func TestVersion227FreshEmptyNoRootSucceeds(t *testing.T) {
	db := openMigrationTestDB(t)
	prefix := fmt.Sprintf("v227empty_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	admin := tableName(cfg, "admin")
	log := tableName(cfg, "admin_log")
	for _, table := range []string{admin, log} {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(admin)+" (id INT PRIMARY KEY, parent_id INT NULL)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(log)+" (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0)").Error)
	require.NoError(t, version227(db, cfg))
}
