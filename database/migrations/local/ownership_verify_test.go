package local

import (
	"strings"
	"testing"

	"go-build-admin/app/pkg/testutil"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

// setupOwnerValidationFixture 在一次性测试库中建出带合法属主列契约的 admin
// 与日志表（独立前缀 ownv_，避免与其它测试的表冲突）。
func setupOwnerValidationFixture(t *testing.T) (*gorm.DB, *conf.Configuration, string, map[string]string) {
	t.Helper()
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = "ownv_"
	adminTable := core.TableName(cfg, "admin")

	drop := func(table string) {
		if err := db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
	createOwnerTable := func(table string) {
		drop(table)
		if err := db.Exec("CREATE TABLE `"+table+"` ("+
			"id bigint unsigned NOT NULL AUTO_INCREMENT PRIMARY KEY,"+
			"admin_id bigint unsigned NOT NULL DEFAULT 0,"+
			"INDEX idx_admin_id (admin_id)"+
			")").Error; err != nil {
			t.Fatal(err)
		}
	}

	createOwnerTable(adminTable)
	if err := db.Exec("INSERT INTO `" + adminTable + "` (id) VALUES (1)").Error; err != nil {
		t.Fatal(err)
	}

	logTables := map[string]string{}
	for _, logical := range []string{"admin_log", "crud_log"} {
		logTables[logical] = core.TableName(cfg, logical)
		createOwnerTable(logTables[logical])
	}
	t.Cleanup(func() {
		for _, table := range logTables {
			_ = db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error
		}
		_ = db.Exec("DROP TABLE IF EXISTS `" + adminTable + "`").Error
	})
	return db, cfg, adminTable, logTables
}

// 匿名行（admin_id=0）：严格校验拒绝、宽容校验接受；
// 非零悬空引用：两种校验都必须拒绝。
func TestValidateMigrationOwnersAnonymousTolerance(t *testing.T) {
	db, _, adminTable, logTables := setupOwnerValidationFixture(t)
	logTable := logTables["admin_log"]

	if err := db.Exec("INSERT INTO `" + logTable + "` (admin_id) VALUES (0)").Error; err != nil {
		t.Fatal(err)
	}
	if err := validateMigrationOwners(db, logTable, adminTable); err == nil {
		t.Fatal("strict validation must reject admin_id=0 rows")
	}
	if err := validateMigrationOwnersAnonymousOK(db, logTable, adminTable); err != nil {
		t.Fatalf("tolerant validation must accept anonymous rows: %v", err)
	}

	if err := db.Exec("INSERT INTO `" + logTable + "` (admin_id) VALUES (999)").Error; err != nil {
		t.Fatal(err)
	}
	if err := validateMigrationOwners(db, logTable, adminTable); err == nil {
		t.Fatal("strict validation must reject dangling non-zero admin_id")
	}
	if err := validateMigrationOwnersAnonymousOK(db, logTable, adminTable); err == nil {
		t.Fatal("tolerant validation must reject dangling non-zero admin_id")
	}
}

// 接线层：verifyOwnerColumns 对 admin_log 使用宽容校验（匿名行通过），
// 其它属主表（crud_log）保持严格（匿名行拒绝）。
func TestVerifyOwnerColumnsAdminLogAnonymousWiring(t *testing.T) {
	db, cfg, _, logTables := setupOwnerValidationFixture(t)

	for _, table := range logTables {
		if err := db.Exec("INSERT INTO `" + table + "` (admin_id) VALUES (0)").Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := verifyOwnerColumns(db, cfg, []string{"admin_log"}); err != nil {
		t.Fatalf("admin_log must tolerate anonymous failed-login rows: %v", err)
	}
	err := verifyOwnerColumns(db, cfg, []string{"crud_log"})
	if err == nil {
		t.Fatal("crud_log must keep rejecting anonymous rows")
	}
	if !strings.Contains(err.Error(), "invalid admin owner") {
		t.Fatalf("crud_log rejection should cite invalid owners, got: %v", err)
	}
}
