package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestAlter(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BUILDADMIN_TEST_MYSQL_DSN not set; skipping DB mutation test")
	}
	db, err := gorm.Open(mysql.Open(dsn))
	require.NoError(t, err)
	comment := "test表"
	if err := db.Exec("ALTER TABLE `"+"ba_test5"+"` COMMENT = ?", comment).Error; err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("成功")
	}
}

func TestGetDDLFieldData_NullableSemantics(t *testing.T) {
	nullable, err := getDDlFieldData(model.Field{Name: "nickname", Type: "varchar", Length: 64, Null: true})
	require.NoError(t, err)
	assert.NotContains(t, nullable, "NOT NULL")

	notNullable, err := getDDlFieldData(model.Field{Name: "status", Type: "int", Null: false})
	require.NoError(t, err)
	assert.Contains(t, notNullable, "NOT NULL")
}

func TestHasColumnMySQL(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BUILDADMIN_TEST_MYSQL_DSN not set; skipping hasColumn integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn))
	require.NoError(t, err)
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
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BUILDADMIN_TEST_MYSQL_DSN not set; skipping primary key integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn))
	require.NoError(t, err)
	tableName := fmt.Sprintf("crud_primary_key_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE TABLE `"+tableName+"` (order_id INT PRIMARY KEY, name VARCHAR(32))").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `" + tableName + "`").Error })
	primaryKey, err := actualPrimaryKey(db, tableName)
	require.NoError(t, err)
	assert.Equal(t, "order_id", primaryKey)
}

func TestDataScopeMySQLIndexProofAndDDL(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BUILDADMIN_TEST_MYSQL_DSN not set; skipping MySQL index integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn))
	require.NoError(t, err)

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
