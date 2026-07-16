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

func TestVersion228SignsOnlyDeltaColumns(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	prefix := fmt.Sprintf("v228_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	money := tableName(cfg, "user_money_log")
	score := tableName(cfg, "user_score_log")
	for _, table := range []string{money, score} {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(money)+" (id INT PRIMARY KEY, money INT UNSIGNED NOT NULL DEFAULT 0, `before` INT UNSIGNED NOT NULL DEFAULT 0, `after` INT UNSIGNED NOT NULL DEFAULT 0)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(score)+" (id INT PRIMARY KEY, score INT UNSIGNED NOT NULL DEFAULT 0, `before` INT UNSIGNED NOT NULL DEFAULT 0, `after` INT UNSIGNED NOT NULL DEFAULT 0)").Error)
	require.NoError(t, version228(db, cfg))
	for table, column := range map[string]string{money: "money", score: "score"} {
		var typ string
		require.NoError(t, db.Raw("SELECT column_type FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, column).Scan(&typ).Error)
		require.NotContains(t, typ, "unsigned")
	}
	for _, column := range []string{"before", "after"} {
		var typ string
		require.NoError(t, db.Raw("SELECT column_type FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", money, column).Scan(&typ).Error)
		require.Contains(t, typ, "unsigned")
	}
}
