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

func openVersion226TestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func TestVersion226BackfillsOwnersAndIsIdempotent(t *testing.T) {
	db := openVersion226TestDB(t)
	prefix := fmt.Sprintf("v226_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	tables := []string{tableName(cfg, "admin"), tableName(cfg, "user"), tableName(cfg, "user_money_log"), tableName(cfg, "user_score_log")}
	for _, table := range tables {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(tables[0])+" (id INT PRIMARY KEY, parent_id INT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(tables[0])+" VALUES (1,NULL),(2,1)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(tables[1])+" (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(tables[2])+" (id INT PRIMARY KEY, user_id INT, admin_id INT UNSIGNED NOT NULL DEFAULT 0, money INT UNSIGNED, `before` INT UNSIGNED, `after` INT UNSIGNED)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(tables[3])+" (id INT PRIMARY KEY, user_id INT, admin_id INT UNSIGNED NOT NULL DEFAULT 0, score INT UNSIGNED, `before` INT UNSIGNED, `after` INT UNSIGNED)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(tables[1])+" VALUES (10,2),(20,0)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(tables[2])+" VALUES (1,10,0,0,0,0),(2,20,0,0,0,0),(3,999,999,0,0,0)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(tables[3])+" VALUES (1,10,0,0,0,0)").Error)
	require.NoError(t, version226(db, cfg))
	var owner int32
	require.NoError(t, db.Table(tables[1]).Where("id=20").Pluck("admin_id", &owner).Error)
	require.Equal(t, int32(1), owner)
	require.NoError(t, db.Table(tables[2]).Where("id=1").Pluck("admin_id", &owner).Error)
	require.Equal(t, int32(2), owner)
	require.NoError(t, db.Table(tables[2]).Where("id=3").Pluck("admin_id", &owner).Error)
	require.Equal(t, int32(1), owner)
	require.NoError(t, version226(db, cfg))
}

func TestVersion226FreshEmptyDoesNotRequireRoot(t *testing.T) {
	db := openVersion226TestDB(t)
	prefix := fmt.Sprintf("v226empty_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	admin := tableName(cfg, "admin")
	user := tableName(cfg, "user")
	for _, table := range []string{admin, user} {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(admin)+" (id INT PRIMARY KEY, parent_id INT NULL)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(user)+" (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0)").Error)
	require.NoError(t, version226(db, cfg))
}

func TestMigrationColumnInfoReadsMySQLMetadataByPosition(t *testing.T) {
	db := openVersion226TestDB(t)
	prefix := fmt.Sprintf("v226meta_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	table := tableName(cfg, "metadata_probe")
	db.Exec("DROP TABLE IF EXISTS " + q(table))
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	require.NoError(t, db.Exec("CREATE TABLE "+q(table)+" (owner_id INT UNSIGNED NOT NULL DEFAULT 0, flag TINYINT UNSIGNED NOT NULL DEFAULT 0)").Error)
	owner, ok, err := migrationColumnInfo(db, table, "owner_id")
	require.NoError(t, err)
	require.True(t, ok)
	require.Contains(t, owner.ColumnType, "int")
	require.Contains(t, owner.ColumnType, "unsigned")
	require.Equal(t, "NO", owner.Nullable)
	require.NotNil(t, owner.Default)
	require.Equal(t, "0", *owner.Default)
	flag, ok, err := migrationColumnInfo(db, table, "flag")
	require.NoError(t, err)
	require.True(t, ok)
	require.Contains(t, flag.ColumnType, "tinyint")
	require.Contains(t, flag.ColumnType, "unsigned")
	require.Equal(t, "NO", flag.Nullable)
	require.NotNil(t, flag.Default)
	require.Equal(t, "0", *flag.Default)
}
