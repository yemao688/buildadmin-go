package migrations

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/testutil"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestFreshSeedPendingRetryAfterOverlayFailure(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = fmt.Sprintf("fresh_retry_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	models := core.CoreModels()
	migrateDB := db.Session(&gorm.Session{NewDB: true})
	require.NoError(t, migrateDB.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(models...))
	t.Cleanup(func() {
		for _, logical := range core.CoreLogicalNames() {
			db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, logical)))
		}
	})
	require.NoError(t, BootstrapOfficialLedger(migrateDB, cfg))
	require.NoError(t, MarkSeedPending(migrateDB, cfg))
	_, err := RunOfficialMigrations(migrateDB, cfg, OfficialMigrations())
	require.NoError(t, err)
	require.NoError(t, migrateDB.Exec("INSERT INTO `"+tableName(cfg, "security_data_recycle")+"` (id,admin_id,name,controller,controller_as,data_table,primary_key) VALUES (1,0,'会员','user/User.php','auth/user','user','id'),(5,0,'会员','user/User.php','user/user','user','id')").Error)
	require.NoError(t, migrateDB.Exec("INSERT INTO `"+tableName(cfg, "security_sensitive_data")+"` (id,admin_id,name,controller,controller_as,data_table,primary_key,data_fields) VALUES (1,0,'会员数据','user/User.php','auth/user','user','id', '{\"username\":\"用户名\",\"mobile\":\"手机号\",\"password\":\"密码\"}'),(2,0,'会员数据','user/User.php','user/user','user','id', '{\"username\":\"用户名\",\"mobile\":\"手机号\"}')").Error)
	lockName := fmt.Sprintf("fresh-seed-%d", os.Getpid())
	require.NoError(t, WithMigrationLock(migrateDB, lockName, time.Second, func(pinned *gorm.DB) error { return RunOfficialFreshSeed(pinned, cfg) }))
	pending, err := SeedPending(migrateDB, cfg)
	require.NoError(t, err)
	require.False(t, pending)
	var baselineRows int64
	require.NoError(t, db.Table(tableName(cfg, "admin")).Count(&baselineRows).Error)
	require.Equal(t, int64(1), baselineRows)
	require.NoError(t, db.Table(tableName(cfg, "security_data_recycle")).Count(&baselineRows).Error)
	require.Equal(t, int64(6), baselineRows)
	require.NoError(t, db.Table(tableName(cfg, "security_sensitive_data")).Count(&baselineRows).Error)
	require.Equal(t, int64(3), baselineRows)
	// MySQL TIMESTAMP columns have second precision; keep the idempotent rerun's update observable.
	time.Sleep(time.Second)
	require.NoError(t, WithMigrationLock(migrateDB, lockName, time.Second, func(pinned *gorm.DB) error { return RunOfficialFreshSeed(pinned, cfg) }))
	pending, err = SeedPending(migrateDB, cfg)
	require.NoError(t, err)
	require.False(t, pending)
	require.NoError(t, BootstrapLocalLedger(migrateDB, cfg))
	_, err = RunLocalMigrations(migrateDB, cfg, OfficialMigrations(), LocalMigrations())
	require.NoError(t, err)
	queryDB := func() *gorm.DB { return db.Session(&gorm.Session{NewDB: true}) }
	require.NoError(t, queryDB().Table(tableName(cfg, "security_data_recycle")).Where("id=1").Count(&baselineRows).Error)
	require.Zero(t, baselineRows)
	require.NoError(t, queryDB().Table(tableName(cfg, "security_data_recycle")).Where("id=5").Count(&baselineRows).Error)
	require.Equal(t, int64(1), baselineRows)
	require.NoError(t, queryDB().Table(tableName(cfg, "security_sensitive_data")).Where("id=1").Count(&baselineRows).Error)
	require.Zero(t, baselineRows)
	require.NoError(t, queryDB().Table(tableName(cfg, "security_sensitive_data")).Where("id=2").Count(&baselineRows).Error)
	require.Equal(t, int64(1), baselineRows)
	require.NoError(t, LocalVerifyCurrent(migrateDB, cfg))
}

func TestUpstreamSecurityBaselineThenLocalOverlay(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = fmt.Sprintf("baseline_overlay_%d_", time.Now().UnixNano())
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	models := core.CoreModels()
	require.NoError(t, db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(models...))
	t.Cleanup(func() {
		for _, logical := range core.CoreLogicalNames() {
			db.Exec("DROP TABLE IF EXISTS " + quoteIdentifier(tableName(cfg, logical)))
		}
	})
	// A current snapshot with empty security rule rows is a valid upgrade state.
	require.NoError(t, LocalMigrations()[3].VerifyUpgradeData(db.Session(&gorm.Session{NewDB: true}), cfg))
	require.NoError(t, NewInstall(db).InsertData())
	require.NoError(t, LocalMigrations()[1].Up(db, cfg))
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
	// 模拟真实编排顺序：ownership-and-audit-integrity（[2]）先于 security 收敛（[3]）执行，
	// 否则 admin_id 不会被归一到 root，与 RunLocalMigrations 的 1→6 顺序对齐。
	require.NoError(t, LocalMigrations()[2].Up(db, cfg))
	require.NoError(t, LocalMigrations()[3].Up(db, cfg))
	require.NoError(t, LocalMigrations()[4].Up(db, cfg))
	require.NoError(t, LocalMigrations()[5].Up(db, cfg))
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
	require.NoError(t, LocalVerifyCurrent(db.Session(&gorm.Session{NewDB: true}), cfg))
}
