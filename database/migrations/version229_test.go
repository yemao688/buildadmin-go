package migrations

import (
	"fmt"
	"os"
	"testing"

	"go-build-admin/conf"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestVersion229TargetOwnerAndIndexContract(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	prefix := fmt.Sprintf("v229_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	admin := tableName(cfg, "admin")
	recycle := tableName(cfg, "security_data_recycle_log")
	for _, table := range []string{admin, recycle} {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(admin)+" (id INT PRIMARY KEY, parent_id INT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(admin)+" VALUES (1,NULL),(2,1)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(recycle)+" (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0, target_admin_id INT UNSIGNED NOT NULL DEFAULT 0, data TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(recycle)+" VALUES (1,0,0,'{\"admin_id\":2}'),(2,0,0,'{}'),(3,0,9,'{}')").Error)
	// An invalid nonzero target owner must fail closed rather than being guessed.
	require.Error(t, version229(db, cfg))
	require.NoError(t, db.Exec("UPDATE "+q(recycle)+" SET target_admin_id=0 WHERE id=3").Error)
	require.NoError(t, version229(db, cfg))
	var target int32
	require.NoError(t, db.Table(recycle).Where("id=1").Pluck("target_admin_id", &target).Error)
	require.Equal(t, int32(2), target)
	require.NoError(t, version229(db, cfg))
	var indexCount int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name='idx_target_admin_id' AND column_name='target_admin_id'", recycle).Scan(&indexCount).Error)
	require.Equal(t, int64(1), indexCount)
}

func TestVersion229AllowsZeroTargetsWithoutAdminTable(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	prefix := fmt.Sprintf("v229empty_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	recycle := tableName(cfg, "security_data_recycle_log")
	db.Exec("DROP TABLE IF EXISTS " + q(recycle))
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(recycle)) })
	require.NoError(t, db.Exec("CREATE TABLE "+q(recycle)+" (id INT PRIMARY KEY, target_admin_id INT UNSIGNED NOT NULL DEFAULT 0, data TEXT)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(recycle)+" VALUES (1,0,'{}')").Error)
	require.NoError(t, version229(db, cfg))
}
