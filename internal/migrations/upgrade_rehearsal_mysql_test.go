package migrations

import (
	"fmt"
	"testing"
	"time"

	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// TestUpgradeRehearsalSelfHealsExistingDatabaseMySQL 是 P0 升级演练的回归测试：
// 在已安装库上删除安全种子、国家菜单与一张核心表，再次 Run 必须通过
// EnsureMissingCoreTables + EnsureSeed 自愈，而不是让 migrate/serve 失败。
func TestUpgradeRehearsalSelfHealsExistingDatabaseMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	prefix := fmt.Sprintf("rehearse_%d_", time.Now().UnixNano())
	cfg.Database.Prefix = prefix
	db = db.Session(&gorm.Session{NewDB: true})
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	cleanup := func() {
		var tables []string
		_ = db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name LIKE ?", prefix+"%").Scan(&tables).Error
		for _, table := range tables {
			_ = db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(table)).Error
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	_, err := Run(db, cfg)
	require.NoError(t, err)

	recycleTable := prefix + "security_data_recycle"
	sensitiveTable := prefix + "security_sensitive_data"
	ruleTable := prefix + "admin_rule"
	languageTable := prefix + "country_language"
	for _, check := range []struct {
		query string
		args  []any
	}{
		{"SELECT COUNT(*) FROM " + quoteIdentifier(recycleTable) + " WHERE id = ?", []any{5}},
		{"SELECT COUNT(*) FROM " + quoteIdentifier(sensitiveTable) + " WHERE id = ?", []any{2}},
		{"SELECT COUNT(*) FROM " + quoteIdentifier(ruleTable) + " WHERE name = ?", []any{"country"}},
	} {
		var count int64
		require.NoError(t, db.Raw(check.query, check.args...).Scan(&count).Error)
		require.Equal(t, int64(1), count, check.query)
	}

	require.NoError(t, db.Exec("DELETE FROM "+quoteIdentifier(recycleTable)+" WHERE id = 5").Error)
	require.NoError(t, db.Exec("DELETE FROM "+quoteIdentifier(sensitiveTable)+" WHERE id = 2").Error)
	require.NoError(t, db.Exec("DELETE FROM "+quoteIdentifier(ruleTable)+" WHERE name = ?", "country").Error)
	require.NoError(t, db.Exec("DROP TABLE "+quoteIdentifier(languageTable)).Error)

	// 第二次 Run 模拟存量库升级：缺失核心表应被补建，安全种子与国家菜单应补回。
	_, err = Run(db, cfg)
	require.NoError(t, err)

	for _, check := range []struct {
		query string
		args  []any
	}{
		{"SELECT COUNT(*) FROM " + quoteIdentifier(recycleTable) + " WHERE id = ?", []any{5}},
		{"SELECT COUNT(*) FROM " + quoteIdentifier(sensitiveTable) + " WHERE id = ?", []any{2}},
		{"SELECT COUNT(*) FROM " + quoteIdentifier(ruleTable) + " WHERE name = ?", []any{"country"}},
		{"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", []any{languageTable}},
	} {
		var count int64
		require.NoError(t, db.Raw(check.query, check.args...).Scan(&count).Error)
		require.Equal(t, int64(1), count, check.query)
	}
}
