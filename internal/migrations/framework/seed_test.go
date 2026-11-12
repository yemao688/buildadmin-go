package framework

import (
	"fmt"
	"testing"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSeedSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:framework-seed-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func seedTestConfig() *conf.Configuration {
	return &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}
}

func createSecuritySeedTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, ddl := range []string{
		"CREATE TABLE IF NOT EXISTS `ba_security_data_recycle` (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '', controller TEXT NOT NULL DEFAULT '', controller_as TEXT NOT NULL DEFAULT '', data_table TEXT NOT NULL DEFAULT '', primary_key TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '1', connection TEXT NOT NULL DEFAULT '', update_time INTEGER, create_time INTEGER)",
		"CREATE TABLE IF NOT EXISTS `ba_security_sensitive_data` (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT '', controller TEXT NOT NULL DEFAULT '', controller_as TEXT NOT NULL DEFAULT '', data_table TEXT NOT NULL DEFAULT '', primary_key TEXT NOT NULL DEFAULT '', data_fields TEXT, status TEXT NOT NULL DEFAULT '1', connection TEXT NOT NULL DEFAULT '', update_time INTEGER, create_time INTEGER)",
	} {
		require.NoError(t, db.Exec(ddl).Error)
	}
}

type seedIdentityRow struct {
	ID           int32  `gorm:"column:id"`
	Name         string `gorm:"column:name"`
	Controller   string `gorm:"column:controller"`
	ControllerAs string `gorm:"column:controller_as"`
	DataTable    string `gorm:"column:data_table"`
	Status       string `gorm:"column:status"`
}

func TestEnsureSecuritySeedRowsSelfHealsWithoutOverwriting(t *testing.T) {
	db := openSeedSQLite(t)
	createSecuritySeedTables(t, db)
	cfg := seedTestConfig()

	require.NoError(t, ensureSecuritySeedRows(db, cfg))
	for _, check := range []struct {
		table, name, controllerAs, dataTable string
		id                                   int32
	}{
		{"ba_security_data_recycle", "会员", "user/user", "user", 5},
		{"ba_security_sensitive_data", "会员数据", "user/user", "user", 2},
	} {
		var row seedIdentityRow
		require.NoError(t, db.Table(check.table).Where("id = ?", check.id).Take(&row).Error)
		require.Equal(t, check.name, row.Name)
		require.Equal(t, "user.User", row.Controller)
		require.Equal(t, check.controllerAs, row.ControllerAs)
		require.Equal(t, check.dataTable, row.DataTable)
		require.Equal(t, "1", row.Status)
	}

	// 重跑不清空、不覆盖、不重复插入。
	require.NoError(t, ensureSecuritySeedRows(db, cfg))
	for _, table := range []string{"ba_security_data_recycle", "ba_security_sensitive_data"} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Equal(t, int64(1), count)
	}

	// 直接删除后由下一次补种自愈（对应常驻 Verify 之前的编排步骤）。
	require.NoError(t, db.Exec("DELETE FROM `ba_security_data_recycle` WHERE id = 5").Error)
	require.NoError(t, db.Exec("DELETE FROM `ba_security_sensitive_data` WHERE id = 2").Error)
	require.NoError(t, ensureSecuritySeedRows(db, cfg))
	var restored int64
	require.NoError(t, db.Table("ba_security_data_recycle").Where("id = 5").Count(&restored).Error)
	require.Equal(t, int64(1), restored)
	require.NoError(t, db.Table("ba_security_sensitive_data").Where("id = 2").Count(&restored).Error)
	require.Equal(t, int64(1), restored)

	// 用户改写过的同 id 行保持原样（绝不用官方身份覆盖已有行）。
	require.NoError(t, db.Exec("UPDATE `ba_security_data_recycle` SET name = '自定义回收' WHERE id = 5").Error)
	require.NoError(t, db.Exec("UPDATE `ba_security_sensitive_data` SET name = '自定义敏感' WHERE id = 2").Error)
	require.NoError(t, ensureSecuritySeedRows(db, cfg))
	var recycle seedIdentityRow
	require.NoError(t, db.Table("ba_security_data_recycle").Where("id = 5").Take(&recycle).Error)
	require.Equal(t, "自定义回收", recycle.Name)
	var sensitive seedIdentityRow
	require.NoError(t, db.Table("ba_security_sensitive_data").Where("id = 2").Take(&sensitive).Error)
	require.Equal(t, "自定义敏感", sensitive.Name)
}

func TestRemoveLegacyModuleStoreMenusIsIdempotent(t *testing.T) {
	db := openSeedSQLite(t)
	require.NoError(t, testutil.CreateSQLiteAdminRuleTables(db, "ba_admin_rule", "ba_admin_group", "ba_admin_group_access"))
	cfg := seedTestConfig()
	for _, row := range []struct {
		pid              int
		typ, title, name string
	}{
		{0, "menu", "模块市场", "moduleStore/moduleStore"},
		{1, "button", "安装", "moduleStore/moduleStore/install"},
		{0, "menu", "CRUD代码生成", "crud/crud"},
	} {
		require.NoError(t, db.Exec("INSERT INTO `ba_admin_rule` (pid, type, title, name) VALUES (?, ?, ?, ?)", row.pid, row.typ, row.title, row.name).Error)
	}
	require.NoError(t, removeLegacyModuleStore(db, cfg))
	var count int64
	require.NoError(t, db.Table("ba_admin_rule").Where("name = ? OR name LIKE ?", "moduleStore/moduleStore", "moduleStore/moduleStore/%").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("ba_admin_rule").Where("name = ?", "crud/crud").Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, removeLegacyModuleStore(db, cfg))
}

func TestSeedCountryMenusDeduplicatesOnParentNameAndType(t *testing.T) {
	db := openSeedSQLite(t)
	require.NoError(t, testutil.CreateSQLiteAdminRuleTables(db, "ba_admin_rule", "ba_admin_group", "ba_admin_group_access"))
	cfg := seedTestConfig()

	// 同名但不同父/类型的规则不能再吞并 country 菜单，也不能被当作父级。
	require.NoError(t, db.Exec("INSERT INTO `ba_admin_rule` (pid, type, title, name) VALUES (999, 'menu', '冒名规则', 'country')").Error)

	require.NoError(t, seedCountryMenus(db, cfg))

	var country struct {
		ID   int32
		Pid  int32
		Type string
	}
	require.NoError(t, db.Table("ba_admin_rule").Where("pid = 0 AND name = 'country' AND type = 'menu_dir'").Take(&country).Error)
	require.NotZero(t, country.ID)

	var impostor int64
	require.NoError(t, db.Table("ba_admin_rule").Where("pid = 999 AND name = 'country' AND type = 'menu'").Count(&impostor).Error)
	require.Equal(t, int64(1), impostor)

	// 子菜单挂在新建的 country 目录下，而不是同名冒名规则下。
	var wrongParent int64
	require.NoError(t, db.Table("ba_admin_rule").Where("name = 'country/currency' AND pid <> ?", country.ID).Count(&wrongParent).Error)
	require.Zero(t, wrongParent)

	var total int64
	require.NoError(t, db.Table("ba_admin_rule").Count(&total).Error)
	// 1 个目录 + 3 个菜单 + 15 个按钮 + 1 条冒名规则。
	require.Equal(t, int64(20), total)

	// 重跑幂等。
	require.NoError(t, seedCountryMenus(db, cfg))
	var after int64
	require.NoError(t, db.Table("ba_admin_rule").Count(&after).Error)
	require.Equal(t, total, after)
}
