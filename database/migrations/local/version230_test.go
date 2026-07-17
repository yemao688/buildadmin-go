package local

import (
	"fmt"
	"go-build-admin/database/migrations/internal/core"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestVersion230MarksUnrecoverableTargets(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	prefix := fmt.Sprintf("v230_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	table := core.TableName(cfg, "security_data_recycle_log")
	db.Exec("DROP TABLE IF EXISTS " + q(table))
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	require.NoError(t, db.Exec("CREATE TABLE "+q(table)+" (id INT PRIMARY KEY, target_admin_id INT UNSIGNED NOT NULL DEFAULT 0)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(table)+" VALUES (1,0),(2,0)").Error)
	require.NoError(t, version230(db, cfg))
	var marked int64
	require.NoError(t, db.Table(table).Where("legacy_unrecoverable=1").Count(&marked).Error)
	require.Equal(t, int64(2), marked)
	require.NoError(t, version230(db, cfg))
}
