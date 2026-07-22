package migrations

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/local"
	"go-build-admin/database/migrations/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestFreshSeedPendingRetryAfterOverlayFailure(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("fresh_retry_%d_", os.Getpid())}}
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	models := []any{
		&model.AdminGroupAccess{}, &model.AdminGroup{}, &model.AdminLog{}, &model.AdminRule{}, &model.Admin{}, &model.AdminClosure{}, &model.AdminHierarchyLock{},
		&model.Area{}, &model.Attachment{}, &model.Captcha{}, &model.Config{}, &model.CrudLog{}, &model.Migrations{},
		&model.SecurityDataRecycleLog{}, &model.SecurityDataRecycle{}, &model.SecuritySensitiveDataLog{}, &model.SecuritySensitiveData{}, &model.TestBuild{}, &model.Token{},
		&model.UserGroup{}, &model.UserMoneyLog{}, &model.UserRule{}, &model.UserScoreLog{}, &model.User{},
	}
	db = db.Set("gorm:table_options", "ENGINE=InnoDB")
	require.NoError(t, db.AutoMigrate(models...))
	t.Cleanup(func() {
		for _, logical := range []string{"admin_group_access", "admin_group", "admin_log", "admin_rule", "admin", "admin_closure", "admin_hierarchy_lock", "area", "attachment", "captcha", "config", "crud_log", "migrations", "security_data_recycle_log", "security_data_recycle", "security_sensitive_data_log", "security_sensitive_data", "test_build", "token", "user_group", "user_money_log", "user_rule", "user_score_log", "user"} {
			db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, logical)))
		}
	})
	require.NoError(t, BootstrapOfficialLedger(db, cfg))
	require.NoError(t, MarkSeedPending(db, cfg))
	require.NoError(t, db.Exec("INSERT INTO `"+tableName(cfg, "security_data_recycle")+"` (id,admin_id,name,controller,controller_as,data_table,primary_key) VALUES (1,0,'会员','user/User.php','auth/user','user','id'),(5,0,'会员','user/User.php','user/user','user','id')").Error)
	require.NoError(t, db.Exec("INSERT INTO `"+tableName(cfg, "security_sensitive_data")+"` (id,admin_id,name,controller,controller_as,data_table,primary_key,data_fields) VALUES (1,0,'会员数据','user/User.php','auth/user','user','id', '{\"username\":\"用户名\",\"mobile\":\"手机号\",\"password\":\"密码\"}'),(2,0,'会员数据','user/User.php','user/user','user','id', '{\"username\":\"用户名\",\"mobile\":\"手机号\"}')").Error)
	failing := []LocalMigration{{ID: "failing-overlay", PostSeedVerify: func(*gorm.DB, *conf.Configuration) error { return fmt.Errorf("simulated snapshot interruption") }}}
	lockName := fmt.Sprintf("fresh-seed-%d", os.Getpid())
	require.NoError(t, WithMigrationLock(db, lockName, time.Second, func(pinned *gorm.DB) error { return RunOfficialFreshSeed(pinned, cfg) }))
	require.Error(t, RunPostSeedVerify(db, cfg, failing))
	pending, err := SeedPending(db, cfg)
	require.NoError(t, err)
	require.False(t, pending)
	var baselineRows int64
	require.NoError(t, db.Table(tableName(cfg, "admin")).Count(&baselineRows).Error)
	require.Zero(t, baselineRows)
	require.NoError(t, db.Table(tableName(cfg, "security_data_recycle")).Count(&baselineRows).Error)
	require.Equal(t, int64(2), baselineRows)
	require.NoError(t, db.Table(tableName(cfg, "security_sensitive_data")).Count(&baselineRows).Error)
	require.Equal(t, int64(2), baselineRows)
	require.NoError(t, WithMigrationLock(db, lockName, time.Second, func(pinned *gorm.DB) error { return RunOfficialFreshSeed(pinned, cfg) }))
	pending, err = SeedPending(db, cfg)
	require.NoError(t, err)
	require.False(t, pending)
	queryDB := func() *gorm.DB { return db.Session(&gorm.Session{NewDB: true}) }
	require.NoError(t, queryDB().Table(tableName(cfg, "security_data_recycle")).Where("id=1").Count(&baselineRows).Error)
	require.Zero(t, baselineRows)
	require.NoError(t, queryDB().Table(tableName(cfg, "security_data_recycle")).Where("id=5").Count(&baselineRows).Error)
	require.Equal(t, int64(1), baselineRows)
	require.NoError(t, queryDB().Table(tableName(cfg, "security_sensitive_data")).Where("id=1").Count(&baselineRows).Error)
	require.Zero(t, baselineRows)
	require.NoError(t, queryDB().Table(tableName(cfg, "security_sensitive_data")).Where("id=2").Count(&baselineRows).Error)
	require.Equal(t, int64(1), baselineRows)
	require.NoError(t, LocalMigrations()[0].PostSeedVerify(db.Session(&gorm.Session{NewDB: true}), cfg))
}

func TestUpstreamSecurityBaselineThenLocalOverlay(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("baseline_overlay_%d_", time.Now().UnixNano())}}
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	models := []any{&model.AdminGroupAccess{}, &model.AdminGroup{}, &model.AdminLog{}, &model.AdminRule{}, &model.Admin{}, &model.AdminClosure{}, &model.AdminHierarchyLock{}, &model.Area{}, &model.Attachment{}, &model.Captcha{}, &model.Config{}, &model.CrudLog{}, &model.Migrations{}, &model.SecurityDataRecycleLog{}, &model.SecurityDataRecycle{}, &model.SecuritySensitiveDataLog{}, &model.SecuritySensitiveData{}, &model.TestBuild{}, &model.Token{}, &model.UserGroup{}, &model.UserMoneyLog{}, &model.UserRule{}, &model.UserScoreLog{}, &model.User{}}
	require.NoError(t, db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(models...))
	t.Cleanup(func() {
		for _, logical := range []string{"admin_group_access", "admin_group", "admin_log", "admin_rule", "admin", "admin_closure", "admin_hierarchy_lock", "area", "attachment", "captcha", "config", "crud_log", "migrations", "security_data_recycle_log", "security_data_recycle", "security_sensitive_data_log", "security_sensitive_data", "test_build", "token", "user_group", "user_money_log", "user_rule", "user_score_log", "user"} {
			db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, logical)))
		}
	})
	// A current snapshot with empty security rule rows is a valid 0010 upgrade state.
	require.NoError(t, LocalMigrations()[9].VerifyUpgradeData(db.Session(&gorm.Session{NewDB: true}), cfg))
	require.NoError(t, NewInstall(db).InsertData())
	type recycleRow struct {
		ID, AdminID                                    int32
		Name, Controller, Route, DataTable, PrimaryKey string
	}
	var recycle []recycleRow
	require.NoError(t, db.Raw("SELECT id,admin_id,name,controller,controller_as AS route,data_table,primary_key FROM `"+tableName(cfg, "security_data_recycle")+"` ORDER BY id").Scan(&recycle).Error)
	require.Len(t, recycle, 6)
	expectedRecycle := []recycleRow{{1, 0, "管理员", "auth/Admin.php", "auth/admin", "admin", "id"}, {2, 0, "管理员日志", "auth/AdminLog.php", "auth/adminlog", "admin_log", "id"}, {3, 0, "菜单规则", "auth/Menu.php", "auth/rule", "admin_rule", "id"}, {4, 0, "系统配置项", "routine/Config.php", "routine/config", "config", "id"}, {5, 0, "会员", "user/User.php", "auth/user", "user", "id"}, {6, 0, "数据回收规则", "security/DataRecycle.php", "security/datarecycle", "security_data_recycle", "id"}}
	require.Equal(t, expectedRecycle, recycle)
	type sensitiveRow struct {
		ID, AdminID                                            int32
		Name, Controller, Route, DataTable, PrimaryKey, Fields string
	}
	var sensitive []sensitiveRow
	require.NoError(t, db.Raw("SELECT id,admin_id,name,controller,controller_as AS route,data_table,primary_key,data_fields AS fields FROM `"+tableName(cfg, "security_sensitive_data")+"` ORDER BY id").Scan(&sensitive).Error)
	require.Len(t, sensitive, 3)
	expectedSensitive := []sensitiveRow{{1, 0, "管理员数据", "auth/Admin.php", "auth/admin", "admin", "id", `{"username":"用户名","mobile":"手机","password":"密码","status":"状态"}`}, {2, 0, "会员数据", "user/User.php", "user/user", "user", "id", `{"username":"用户名","mobile":"手机号","password":"密码","status":"状态","email":"邮箱地址"}`}, {3, 0, "管理员权限", "auth/Group.php", "auth/group", "admin_group", "id", `{"rules":"权限规则ID"}`}}
	require.Equal(t, expectedSensitive, sensitive)
	require.NoError(t, local.ApplyFreshOverlay(db, cfg))
	var count int64
	require.NoError(t, db.Session(&gorm.Session{NewDB: true}).Table(tableName(cfg, "security_data_recycle")).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, db.Session(&gorm.Session{NewDB: true}).Table(tableName(cfg, "security_data_recycle")).Where("id=5 AND controller_as='user/user' AND data_table='user' AND admin_id=1").Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, db.Session(&gorm.Session{NewDB: true}).Table(tableName(cfg, "security_sensitive_data")).Where("id=1 OR id=3").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Session(&gorm.Session{NewDB: true}).Table(tableName(cfg, "security_sensitive_data")).Where("id=2 AND name='会员数据' AND controller='user/User.php' AND controller_as='user/user' AND data_table='user' AND admin_id=1").Count(&count).Error)
	require.Equal(t, int64(1), count)
	var fields string
	require.NoError(t, db.Session(&gorm.Session{NewDB: true}).Table(tableName(cfg, "security_sensitive_data")).Where("id=2").Pluck("data_fields", &fields).Error)
	require.NotContains(t, fields, "password")
	require.NoError(t, LocalMigrations()[0].PostSeedVerify(db.Session(&gorm.Session{NewDB: true}), cfg))
}
