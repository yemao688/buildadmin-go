package crud_helper

import (
	"fmt"
	adminauth "go-build-admin/app/admin/model/auth"
	crudmodel "go-build-admin/app/admin/model/crud"
	"go-build-admin/app/pkg/testutil"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlter(t *testing.T) {
	db, _ := testutil.OpenMySQL(t)
	comment := "test表"
	if err := db.Exec("ALTER TABLE `"+"ba_test5"+"` COMMENT = ?", comment).Error; err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("成功")
	}
}

func TestGetDDLFieldData_NullableSemantics(t *testing.T) {
	nullable, err := getDDlFieldData(crudmodel.Field{Name: "nickname", Type: "varchar", Length: 64, Null: true})
	require.NoError(t, err)
	assert.NotContains(t, nullable, "NOT NULL")

	notNullable, err := getDDlFieldData(crudmodel.Field{Name: "status", Type: "int", Null: false})
	require.NoError(t, err)
	assert.Contains(t, notNullable, "NOT NULL")
}

func TestGetDDLFieldDataDefaultTypes(t *testing.T) {
	for _, tc := range []struct {
		name, defaultType, value, want string
	}{
		{"none", "NONE", "ignored", "NOT NULL"},
		{"null", "NULL", "", "DEFAULT NULL"},
		{"empty", "EMPTY STRING", "", "DEFAULT ''"},
		{"input", "INPUT", "", "DEFAULT ''"},
		{"legacy_null", "", "null", "DEFAULT NULL"},
		{"legacy_empty", "", "empty string", "DEFAULT ''"},
		{"stale_none", "NONE", "stale", "NOT NULL"},
		{"stale_null", "NULL", "stale", "DEFAULT NULL"},
		{"stale_empty", "EMPTY STRING", "stale", "DEFAULT ''"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getDDlFieldData(crudmodel.Field{Name: tc.name, Type: "varchar", Length: 32, DefaultType: tc.defaultType, Default: tc.value})
			require.NoError(t, err)
			assert.Contains(t, got, tc.want)
		})
	}
}

func TestGetDDLFieldDataNullDefaultIsNullable(t *testing.T) {
	got, err := getDDlFieldData(crudmodel.Field{Name: "value", Type: "varchar", DefaultType: "NULL"})
	require.NoError(t, err)
	assert.NotContains(t, got, "NOT NULL")
	assert.Contains(t, got, "DEFAULT NULL")
}

func TestGetDDLFieldDataNoDefaultFamilies(t *testing.T) {
	for dataType := range noDefaultValueTypes {
		t.Run(dataType, func(t *testing.T) {
			got, err := getDDlFieldData(crudmodel.Field{Name: "value", Type: dataType, DefaultType: "INPUT", Default: "x"})
			require.NoError(t, err)
			assert.NotContains(t, got, "DEFAULT")
		})
	}
}

func TestHasColumnMySQL(t *testing.T) {
	db, _ := testutil.OpenMySQL(t)
	tableName := fmt.Sprintf("crud_has_column_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE TABLE `"+tableName+"` (id INT PRIMARY KEY, name VARCHAR(32))").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `" + tableName + "`").Error })

	exists, err := hasColumn(db, tableName, "name")
	require.NoError(t, err)
	assert.True(t, exists)
	exists, err = hasColumn(db, tableName, "missing")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestActualPrimaryKeyMySQL(t *testing.T) {
	db, _ := testutil.OpenMySQL(t)
	tableName := fmt.Sprintf("crud_primary_key_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE TABLE `"+tableName+"` (order_id INT PRIMARY KEY, name VARCHAR(32))").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `" + tableName + "`").Error })
	primaryKey, err := actualPrimaryKey(db, tableName)
	require.NoError(t, err)
	assert.Equal(t, "order_id", primaryKey)
}

func TestMenuRuleSnapshotRestoreMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = "ba_"
	require.NoError(t, db.Table("ba_admin_rule").AutoMigrate(&adminauth.AdminRule{}))
	menuName := fmt.Sprintf("oracle_menu_snapshot_%d", time.Now().UnixNano())
	rows := []adminauth.AdminRule{
		{Name: menuName, Path: menuName, Title: "snapshot", Type: "menu", Status: "1"},
		{Name: menuName + "/index", Path: menuName + "/index", Title: "view", Type: "button", Status: "1"},
	}
	for i := range rows {
		require.NoError(t, db.Table("ba_admin_rule").Create(&rows[i]).Error)
	}
	t.Cleanup(func() {
		_ = db.Table("ba_admin_rule").Where("name LIKE ?", menuName+"%").Delete(&adminauth.AdminRule{}).Error
	})
	snapshot, err := snapshotMenuRules(db, cfg, menuName)
	require.NoError(t, err)
	require.Len(t, snapshot, 2)
	require.NoError(t, db.Table("ba_admin_rule").Where("name LIKE ?", menuName+"%").Delete(&adminauth.AdminRule{}).Error)
	require.NoError(t, restoreMenuRules(db, cfg, snapshot))
	var count int64
	require.NoError(t, db.Table("ba_admin_rule").Where("name LIKE ?", menuName+"%").Count(&count).Error)
	assert.Equal(t, int64(2), count)
}

func TestDataScopeMySQLIndexProofAndDDL(t *testing.T) {
	db, _ := testutil.OpenMySQL(t)

	newTable := func(t *testing.T, suffix string) string {
		t.Helper()
		name := "crud_scope_idx_" + suffix + "_" + fmt.Sprint(time.Now().UnixNano())
		name = strings.ToLower(name)
		require.NoError(t, db.Exec("CREATE TABLE `"+name+"` (id BIGINT PRIMARY KEY, admin_id BIGINT NOT NULL, status INT NOT NULL)").Error)
		t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `" + name + "`").Error })
		return name
	}

	t.Run("single owner index proves", func(t *testing.T) {
		name := newTable(t, "single")
		require.NoError(t, db.Exec("CREATE INDEX `idx_owner_single` ON `"+name+"` (`admin_id`)").Error)
		ok, err := buildIndexProver(db, name)("admin_id")
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("owner leading composite index proves", func(t *testing.T) {
		name := newTable(t, "leading")
		require.NoError(t, db.Exec("CREATE INDEX `idx_owner_leading` ON `"+name+"` (`admin_id`, `status`)").Error)
		ok, err := buildIndexProver(db, name)("admin_id")
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("owner non-leading composite index does not prove", func(t *testing.T) {
		name := newTable(t, "nonleading")
		require.NoError(t, db.Exec("CREATE INDEX `idx_status_owner` ON `"+name+"` (`status`, `admin_id`)").Error)
		ok, err := buildIndexProver(db, name)("admin_id")
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("same deterministic index name with wrong first column errors", func(t *testing.T) {
		name := newTable(t, "wrongname")
		require.NoError(t, db.Exec("CREATE INDEX `idx_admin_id` ON `"+name+"` (`status`)").Error)
		err := EnsureDataScopeIndex(db, name, "admin_id", "id")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "first column")
	})

	t.Run("missing owner index is created with owner as seq one", func(t *testing.T) {
		name := newTable(t, "create")
		require.NoError(t, EnsureDataScopeIndex(db, name, "admin_id", "id"))
		var firstColumn string
		require.NoError(t, db.Raw(
			"SELECT COLUMN_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ? AND SEQ_IN_INDEX = 1",
			name, "idx_admin_id",
		).Scan(&firstColumn).Error)
		assert.Equal(t, "admin_id", firstColumn)
	})
}
