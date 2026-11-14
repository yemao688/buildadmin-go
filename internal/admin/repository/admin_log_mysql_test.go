package repository

import (
	"context"
	"fmt"
	"os"
	"testing"

	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

// MySQL 门禁用例：验证明细表在真实 MySQL 上的插入路径（sqlite 单测覆盖不到
// int unsigned/索引等方言差异）。未配置 mysql_test 时 SKIP。
func TestAdminLogCreateMySQL(t *testing.T) {
	t.Parallel()
	prefix := fmt.Sprintf("alog_it_%d_", os.Getpid())
	db, config := testutil.OpenMySQL(t)
	config.Database.Prefix = prefix
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	table := prefix + "admin_log"
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS `"+table+"`").Error)
	require.NoError(t, db.Exec("CREATE TABLE `"+table+"` ("+
		"id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,"+
		"admin_id INT UNSIGNED NOT NULL DEFAULT 0,"+
		"username VARCHAR(20) NOT NULL DEFAULT '',"+
		"url VARCHAR(1500) NOT NULL DEFAULT '',"+
		"title VARCHAR(100) NOT NULL DEFAULT '',"+
		"data LONGTEXT NULL,"+
		"ip VARCHAR(50) NOT NULL DEFAULT '',"+
		"useragent VARCHAR(255) NOT NULL DEFAULT '',"+
		"create_time BIGINT UNSIGNED NULL DEFAULT NULL)").Error)
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS `" + table + "`")
	})

	m := NewAdminLogRepository(db, config)
	require.NoError(t, m.Create(context.Background(), &model.AdminLog{
		AdminID:   2,
		Username:  "mysql-user",
		URL:       "/admin/auth.Admin/add",
		Title:     "管理员-新增",
		Data:      `{"name":"alice"}`,
		IP:        "203.0.113.9",
		Useragent: "mysql-agent",
	}))

	var row model.AdminLog
	require.NoError(t, db.Last(&row).Error)
	require.Equal(t, int32(2), row.AdminID)
	require.Equal(t, "mysql-user", row.Username)
	require.Equal(t, "管理员-新增", row.Title)
}
