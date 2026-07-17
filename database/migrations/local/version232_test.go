package local

import (
	"fmt"
	"go-build-admin/database/migrations/internal/core"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestVersion232NormalizesOnlyInstallerRules(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	prefix := fmt.Sprintf("v232_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	recycle := core.TableName(cfg, "security_data_recycle")
	sensitive := core.TableName(cfg, "security_sensitive_data")
	for _, table := range []string{recycle, sensitive} {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q(recycle)+" (id INT PRIMARY KEY, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50), admin_id INT NOT NULL DEFAULT 1)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(sensitive)+" (id INT PRIMARY KEY, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50), data_fields TEXT, admin_id INT NOT NULL DEFAULT 1)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(recycle)+" VALUES (1,'管理员','auth/Admin.php','auth/admin','admin','id',1),(5,'会员','user/User.php','auth/user','user','id',1),(9,'自定义','custom.php','custom/changed','user','id',1)").Error)
	adminFields := `{"username":"用户名","mobile":"手机","password":"密码","status":"状态"}`
	userOldFields := `{"username":"用户名","mobile":"手机号","password":"密码","status":"状态","email":"邮箱地址"}`
	require.NoError(t, db.Exec("INSERT INTO "+q(sensitive)+" VALUES (1,'管理员数据','auth/Admin.php','auth/admin','admin','id',?,1),(2,'会员数据','user/User.php','user/user','user','id',?,1),(3,'自定义','custom.php','custom/changed','user','id','{}',1)", adminFields, userOldFields).Error)

	require.NoError(t, version232(db, cfg))
	var count int64
	require.NoError(t, db.Table(recycle).Where("id=1").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table(recycle).Where("id=5").Count(&count).Error)
	require.Equal(t, int64(1), count)
	var route string
	require.NoError(t, db.Table(recycle).Where("id=5").Pluck("controller_as", &route).Error)
	require.Equal(t, "user/user", route)
	require.NoError(t, db.Table(recycle).Where("id=9").Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, db.Table(sensitive).Where("id=1").Count(&count).Error)
	require.Zero(t, count)
	var fields string
	require.NoError(t, db.Table(sensitive).Where("id=2").Pluck("data_fields", &fields).Error)
	require.NotContains(t, fields, "password")
	require.NoError(t, db.Table(sensitive).Where("id=3").Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, version232(db, cfg))
}

func TestVersion232FreshEmptyPrefixSucceeds(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	prefix := fmt.Sprintf("v232empty_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	q := func(s string) string { return "`" + s + "`" }
	for _, logical := range []string{"security_data_recycle", "security_sensitive_data"} {
		table := core.TableName(cfg, logical)
		db.Exec("DROP TABLE IF EXISTS " + q(table))
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + q(table)) })
		if logical == "security_data_recycle" {
			require.NoError(t, db.Exec("CREATE TABLE "+q(table)+" (id INT PRIMARY KEY, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50), admin_id INT UNSIGNED NOT NULL DEFAULT 1)").Error)
		} else {
			require.NoError(t, db.Exec("CREATE TABLE "+q(table)+" (id INT PRIMARY KEY, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50), data_fields TEXT, admin_id INT UNSIGNED NOT NULL DEFAULT 1)").Error)
		}
	}
	require.NoError(t, version232(db, cfg))
}

func TestVersion232LegacySensitiveUserSignatureIsRejectedAndAdoptionFailsClosed(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("v232legacy_%d_", time.Now().UnixNano())}}
	q := func(table string) string { return core.QuoteIdentifier(core.TableName(cfg, table)) }
	for _, logical := range []string{"admin", "security_data_recycle_log", "security_sensitive_data_log", "security_data_recycle", "security_sensitive_data", "migrations", "go_migrations"} {
		db.Exec("DROP TABLE IF EXISTS " + q(logical))
		table := q(logical)
		t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + table) })
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q("admin")+" (id INT UNSIGNED PRIMARY KEY)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("admin")+" VALUES (1)").Error)
	for _, logical := range []string{"security_data_recycle_log", "security_sensitive_data_log"} {
		require.NoError(t, db.Exec("CREATE TABLE "+q(logical)+" (id INT PRIMARY KEY, target_admin_id INT UNSIGNED NOT NULL DEFAULT 0, legacy_unrecoverable TINYINT(1) UNSIGNED NOT NULL DEFAULT 0, is_committed TINYINT(1) UNSIGNED NOT NULL DEFAULT 0, KEY idx_target_admin_id (target_admin_id)) ENGINE=InnoDB").Error)
	}
	require.NoError(t, db.Exec("CREATE TABLE "+q("security_data_recycle")+" (id INT PRIMARY KEY, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50), admin_id INT UNSIGNED NOT NULL DEFAULT 0)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q("security_sensitive_data")+" (id INT PRIMARY KEY, name VARCHAR(50), controller VARCHAR(100), controller_as VARCHAR(100), data_table VARCHAR(100), primary_key VARCHAR(50), data_fields TEXT, admin_id INT UNSIGNED NOT NULL DEFAULT 0)").Error)
	oldFields := version232SensitiveUserOldFields
	require.NoError(t, db.Exec("INSERT INTO "+q("security_sensitive_data")+" (id,name,controller,controller_as,data_table,primary_key,data_fields,admin_id) VALUES (2,'会员数据','user/User.php','user/user','user','id',?,0)", oldFields).Error)

	// The complete minimum Version231/232 flags are present, so rejection must
	// come from the concrete legacy seed signature rather than missing schema.
	require.Error(t, verifySecurityRuleContract(db.Session(&gorm.Session{NewDB: true}), cfg))
	require.NoError(t, core.BootstrapOfficialLedger(db, cfg))
	require.NoError(t, core.BootstrapLocalLedger(db, cfg))
	finished := time.Now().Add(-time.Minute)
	require.NoError(t, db.Table(core.TableName(cfg, "migrations")).Create(&core.MigrationRecord{Version: 20260722000000, MigrationName: "Version232", StartTime: finished, EndTime: &finished}).Error)
	local := Migrations(nil)[9]
	count, err := core.AdoptCompletedLegacyAliases(db, cfg, []core.LocalMigration{local})
	require.Error(t, err)
	require.Zero(t, count)
	var ledgerCount int64
	require.NoError(t, db.Table(core.TableName(cfg, "go_migrations")).Where("sequence = ?", local.Sequence).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)

	require.NoError(t, version232(db, cfg))
	require.NoError(t, verifySecurityRuleContract(db.Session(&gorm.Session{NewDB: true}), cfg))
	var fields string
	require.NoError(t, db.Table(core.TableName(cfg, "security_sensitive_data")).Where("id = 2").Pluck("data_fields", &fields).Error)
	require.NotContains(t, fields, "password")
}
