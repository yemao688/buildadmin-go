package local

import (
	"go-build-admin/database/migrations/internal/core"
	"os"
	"testing"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func openHierarchyTestDB(t *testing.T, prefix string) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   prefix,
		},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}

func hierarchyConfig(prefix string) *conf.Configuration {
	config := &conf.Configuration{}
	config.Database.Prefix = prefix
	return config
}

func dropHierarchyTables(t *testing.T, db *gorm.DB, config *conf.Configuration) {
	t.Helper()
	_ = db.Migrator().DropTable(core.TableName(config, "admin_hierarchy_lock"))
	_ = db.Migrator().DropTable(core.TableName(config, "admin_closure"))
	_ = db.Migrator().DropTable(core.TableName(config, "admin"))
}

func countClosureRows(t *testing.T, db *gorm.DB, config *conf.Configuration) int64 {
	t.Helper()
	var count int64
	if err := db.Table(core.TableName(config, "admin_closure")).Count(&count).Error; err != nil {
		t.Fatalf("count closure rows: %v", err)
	}
	return count
}

func TestHierarchyMigrationModelNoAfterCreateHook(t *testing.T) {
	db := openHierarchyTestDB(t, "go_")
	config := hierarchyConfig("go_")
	dropHierarchyTables(t, db, config)

	if err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&model.Admin{}, &model.AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	a := &model.Admin{ID: 100, Username: "nohook", Status: "enable", Password: ""}
	if err := db.Create(a).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	if got := countClosureRows(t, db, config); got != 0 {
		t.Fatalf("closure rows = %d, want 0 (AfterCreate hook must not exist)", got)
	}
}

func TestHierarchyVersion224CustomPrefix(t *testing.T) {
	prefix := "cust_"
	db := openHierarchyTestDB(t, prefix)
	config := hierarchyConfig(prefix)
	dropHierarchyTables(t, db, config)

	if err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&model.Admin{}, &model.AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	admins := []*model.Admin{
		{ID: 1, Username: "c1", Status: "enable", Password: ""},
		{ID: 2, Username: "c2", Status: "enable", Password: ""},
	}
	if err := db.Create(admins).Error; err != nil {
		t.Fatalf("create admins: %v", err)
	}

	if err := version224(db, config); err != nil {
		t.Fatalf("version224: %v", err)
	}

	if got := countClosureRows(t, db, config); got != 2 {
		t.Fatalf("closure rows = %d, want 2", got)
	}

	if !core.TableExists(db, core.TableName(config, "admin_closure")) {
		t.Fatal("closure table with custom prefix does not exist")
	}
}

func TestHierarchyVersion224LegacyUpgrade(t *testing.T) {
	prefix := "leg_"
	db := openHierarchyTestDB(t, prefix)
	config := hierarchyConfig(prefix)
	dropHierarchyTables(t, db, config)

	adminTable := core.TableName(config, "admin")
	if err := db.Exec(
		"CREATE TABLE " + core.QuoteIdentifier(adminTable) + " (" +
			"`id` int(11) unsigned NOT NULL AUTO_INCREMENT," +
			"`username` varchar(20) NOT NULL DEFAULT ''," +
			"`status` varchar(30) NOT NULL DEFAULT 'enable'," +
			"PRIMARY KEY (`id`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
	).Error; err != nil {
		t.Fatalf("create legacy admin table: %v", err)
	}

	if err := db.Exec("INSERT INTO "+core.QuoteIdentifier(adminTable)+" (id, username) VALUES (?, ?), (?, ?)", 1, "a1", 2, "a2").Error; err != nil {
		t.Fatalf("seed legacy admins: %v", err)
	}

	if err := version224(db, config); err != nil {
		t.Fatalf("version224: %v", err)
	}

	if !core.ColumnExists(db, adminTable, "parent_id") {
		t.Fatal("parent_id column not created")
	}
	if !core.IndexExists(db, adminTable, "idx_parent_id") {
		t.Fatal("idx_parent_id not created")
	}
	if !core.TableExists(db, core.TableName(config, "admin_closure")) {
		t.Fatal("closure table not created")
	}
	if got := countClosureRows(t, db, config); got != 2 {
		t.Fatalf("closure rows = %d, want 2", got)
	}

	var nullParents int64
	if err := db.Table(adminTable).Where("parent_id IS NOT NULL").Count(&nullParents).Error; err != nil {
		t.Fatalf("count non-null parents: %v", err)
	}
	if nullParents != 0 {
		t.Fatalf("legacy admins must have parent_id=NULL, got %d non-null", nullParents)
	}
}

func TestHierarchyVersion224PartialRecovery(t *testing.T) {
	prefix := "pr_"
	db := openHierarchyTestDB(t, prefix)
	config := hierarchyConfig(prefix)
	dropHierarchyTables(t, db, config)

	if err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&model.Admin{}, &model.AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	admins := []*model.Admin{
		{ID: 1, Username: "p1", Status: "enable", Password: ""},
		{ID: 2, Username: "p2", Status: "enable", Password: ""},
		{ID: 3, Username: "p3", Status: "enable", Password: ""},
	}
	if err := db.Create(admins).Error; err != nil {
		t.Fatalf("create admins: %v", err)
	}

	closureTable := core.TableName(config, "admin_closure")
	if err := db.Exec(
		"INSERT INTO "+core.QuoteIdentifier(closureTable)+" (ancestor_id, descendant_id, depth) VALUES (?, ?, 0)",
		1, 1,
	).Error; err != nil {
		t.Fatalf("seed partial self row: %v", err)
	}

	adminTable := core.TableName(config, "admin")
	if err := db.Exec("DROP INDEX `idx_parent_id` ON " + core.QuoteIdentifier(adminTable)).Error; err != nil {
		t.Fatalf("drop parent index: %v", err)
	}

	if err := version224(db, config); err != nil {
		t.Fatalf("version224 partial recovery: %v", err)
	}

	if got := countClosureRows(t, db, config); got != 3 {
		t.Fatalf("closure rows = %d, want 3", got)
	}
	if !core.IndexExists(db, adminTable, "idx_parent_id") {
		t.Fatal("idx_parent_id not recovered")
	}

	if err := version224(db, config); err != nil {
		t.Fatalf("version224 repeat: %v", err)
	}
	if got := countClosureRows(t, db, config); got != 3 {
		t.Fatalf("closure rows after repeat = %d, want 3", got)
	}
}

func TestHierarchyEnsureAdminClosureSelfRows(t *testing.T) {
	prefix := "ens_"
	db := openHierarchyTestDB(t, prefix)
	config := hierarchyConfig(prefix)
	dropHierarchyTables(t, db, config)

	if err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&model.Admin{}, &model.AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	admins := []*model.Admin{
		{ID: 7, Username: "e7", Status: "enable", Password: ""},
		{ID: 8, Username: "e8", Status: "enable", Password: ""},
	}
	if err := db.Create(admins).Error; err != nil {
		t.Fatalf("create admins: %v", err)
	}

	if err := EnsureAdminClosureSelfRows(db, config); err != nil {
		t.Fatalf("ensure self rows: %v", err)
	}
	if got := countClosureRows(t, db, config); got != 2 {
		t.Fatalf("closure rows = %d, want 2", got)
	}
	if err := EnsureAdminClosureSelfRows(db, config); err != nil {
		t.Fatalf("ensure self rows repeat: %v", err)
	}
	if got := countClosureRows(t, db, config); got != 2 {
		t.Fatalf("closure rows after repeat = %d, want 2", got)
	}
}

func TestHierarchyVersion224FailsWithoutClosureTable(t *testing.T) {
	prefix := "fail_"
	db := openHierarchyTestDB(t, prefix)
	config := hierarchyConfig(prefix)
	_ = db.Migrator().DropTable(core.TableName(config, "admin"))

	adminTable := core.TableName(config, "admin")
	if err := db.Exec(
		"CREATE TABLE " + core.QuoteIdentifier(adminTable) + " (" +
			"`id` int(11) unsigned NOT NULL AUTO_INCREMENT," +
			"`username` varchar(20) NOT NULL DEFAULT ''," +
			"PRIMARY KEY (`id`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
	).Error; err != nil {
		t.Fatalf("create admin table: %v", err)
	}

	if err := EnsureAdminClosureSelfRows(db, config); err == nil {
		t.Fatal("expected error when closure table is missing")
	} else {
		t.Logf("expected error: %v", err)
	}
}

func assertLockTableRow(t *testing.T, db *gorm.DB, config *conf.Configuration) {
	t.Helper()
	lockTable := core.TableName(config, "admin_hierarchy_lock")
	if !core.TableExists(db, lockTable) {
		t.Fatalf("lock table %s does not exist", lockTable)
	}
	var count int64
	if err := db.Table(lockTable).Where("id = ?", 1).Count(&count).Error; err != nil {
		t.Fatalf("count lock row: %v", err)
	}
	if count != 1 {
		t.Fatalf("lock row count = %d, want 1", count)
	}
}

func TestHierarchyVersion224CreatesLockTableAndRow(t *testing.T) {
	prefix := "lck_"
	db := openHierarchyTestDB(t, prefix)
	config := hierarchyConfig(prefix)
	dropHierarchyTables(t, db, config)

	if err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&model.Admin{}, &model.AdminClosure{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	if err := version224(db, config); err != nil {
		t.Fatalf("version224: %v", err)
	}

	assertLockTableRow(t, db, config)
}

func TestHierarchyVersion224LockTableRecovery(t *testing.T) {
	prefix := "lckdrop_"
	db := openHierarchyTestDB(t, prefix)
	config := hierarchyConfig(prefix)
	dropHierarchyTables(t, db, config)

	if err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&model.Admin{}, &model.AdminClosure{}, &model.AdminHierarchyLock{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	if err := db.Migrator().DropTable(core.TableName(config, "admin_hierarchy_lock")); err != nil {
		t.Fatalf("drop lock table: %v", err)
	}

	if err := version224(db, config); err != nil {
		t.Fatalf("version224: %v", err)
	}

	assertLockTableRow(t, db, config)
}

func TestHierarchyVersion224LockRowRecovery(t *testing.T) {
	prefix := "lckrow_"
	db := openHierarchyTestDB(t, prefix)
	config := hierarchyConfig(prefix)
	dropHierarchyTables(t, db, config)

	if err := db.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(&model.Admin{}, &model.AdminClosure{}, &model.AdminHierarchyLock{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	lockTable := core.TableName(config, "admin_hierarchy_lock")
	if err := db.Exec("DELETE FROM " + core.QuoteIdentifier(lockTable)).Error; err != nil {
		t.Fatalf("delete lock row: %v", err)
	}

	if err := version224(db, config); err != nil {
		t.Fatalf("version224: %v", err)
	}

	assertLockTableRow(t, db, config)
}
