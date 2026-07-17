package local

import (
	"fmt"
	"os"
	"testing"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"go-build-admin/database/migrations/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func openCanonicalTestDB(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("local0011_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + core.QuoteIdentifier(core.TableName(cfg, "attachment"))) })
	return db, cfg
}

func TestCanonicalColumnCorrectPositionSkipsAndRetries(t *testing.T) {
	db, cfg := openCanonicalTestDB(t)
	table := core.QuoteIdentifier(core.TableName(cfg, "attachment"))
	if err := db.Exec("CREATE TABLE " + table + " (id INT PRIMARY KEY, admin_id INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '上传管理员ID', user_id INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '上传用户ID', topic VARCHAR(20) NOT NULL DEFAULT '' COMMENT '细目', label VARCHAR(20) NOT NULL) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	spec := canonicalColumns[1]
	if err := applyCanonicalColumn(db, cfg, spec); err != nil {
		t.Fatal(err)
	}
	if err := applyCanonicalColumn(db, cfg, spec); err != nil {
		t.Fatal(err)
	}
	definition, ok, err := core.MigrationColumnInfo(db, core.TableName(cfg, "attachment"), "admin_id")
	if err != nil || !ok || definition.Ordinal != 2 || definition.Default == nil || *definition.Default != "0" || definition.Comment != "上传管理员ID" {
		t.Fatalf("admin_id definition=%#v ok=%v err=%v", definition, ok, err)
	}
}

func TestCanonicalColumnReordersWrongPositionAndPreservesType(t *testing.T) {
	db, cfg := openCanonicalTestDB(t)
	table := core.QuoteIdentifier(core.TableName(cfg, "attachment"))
	if err := db.Exec("CREATE TABLE " + table + " (id INT PRIMARY KEY, topic VARCHAR(20) NOT NULL DEFAULT '' COMMENT '细目', admin_id INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '上传管理员ID', user_id INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '上传用户ID', label VARCHAR(20) NOT NULL) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := applyCanonicalColumn(db, cfg, canonicalColumns[1]); err != nil {
		t.Fatal(err)
	}
	definition, ok, err := core.MigrationColumnInfo(db, core.TableName(cfg, "attachment"), "admin_id")
	if err != nil || !ok || definition.Ordinal != 2 || definition.ColumnType != "int unsigned" && definition.ColumnType != "int(11) unsigned" {
		t.Fatalf("admin_id was not reordered with type preserved: %#v ok=%v err=%v", definition, ok, err)
	}
}

func TestCanonicalColumnMissingFailsClosed(t *testing.T) {
	db, cfg := openCanonicalTestDB(t)
	if err := db.Exec("CREATE TABLE " + core.QuoteIdentifier(core.TableName(cfg, "attachment")) + " (id INT PRIMARY KEY) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := applyCanonicalColumn(db, cfg, canonicalColumns[1]); err == nil {
		t.Fatal("missing canonical column was accepted")
	}
}

func TestCanonicalColumnUnsafeDefinitionFailsWithoutModify(t *testing.T) {
	db, cfg := openCanonicalTestDB(t)
	table := core.QuoteIdentifier(core.TableName(cfg, "attachment"))
	for name, definition := range map[string]string{
		"wrong type":     "VARCHAR(20) NOT NULL DEFAULT '' COMMENT '上传管理员ID'",
		"wrong nullable": "INT UNSIGNED NULL DEFAULT 0 COMMENT '上传管理员ID'",
		"wrong default":  "INT UNSIGNED NOT NULL DEFAULT 1 COMMENT '上传管理员ID'",
	} {
		t.Run(name, func(t *testing.T) {
			db.Exec("DROP TABLE IF EXISTS " + table)
			if err := db.Exec("CREATE TABLE " + table + " (id INT PRIMARY KEY, admin_id " + definition + ", user_id INT UNSIGNED NOT NULL DEFAULT 0, topic VARCHAR(20) NOT NULL DEFAULT '') ENGINE=InnoDB").Error; err != nil {
				t.Fatal(err)
			}
			before, ok, err := core.MigrationColumnInfo(db, core.TableName(cfg, "attachment"), "admin_id")
			if err != nil || !ok {
				t.Fatalf("before definition=%#v ok=%v err=%v", before, ok, err)
			}
			if err := applyCanonicalColumn(db, cfg, canonicalColumns[1]); err == nil {
				t.Fatal("unsafe canonical definition was accepted")
			}
			after, ok, err := core.MigrationColumnInfo(db, core.TableName(cfg, "attachment"), "admin_id")
			if err != nil || !ok || after.ColumnType != before.ColumnType || after.Nullable != before.Nullable || canonicalTestDefault(after) != canonicalTestDefault(before) {
				t.Fatalf("unsafe definition changed: before=%#v after=%#v ok=%v err=%v", before, after, ok, err)
			}
		})
	}
}

func canonicalTestDefault(column core.MigrationColumn) string {
	if column.Default == nil {
		return "<NULL>"
	}
	return *column.Default
}

func TestCanonicalColumnAllowsHistoricalSecurityCommentAndPreservesIndexData(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("local0011_history_%d_", time.Now().UnixNano())}}
	name := core.TableName(cfg, "security_data_recycle_log")
	table := core.QuoteIdentifier(name)
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS " + table) })
	if err := db.Exec("CREATE TABLE " + table + " (id INT PRIMARY KEY, target_admin_id INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '目标数据管理员', admin_id INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '管理员ID', legacy_unrecoverable TINYINT(1) UNSIGNED NOT NULL DEFAULT 0, is_committed TINYINT(1) UNSIGNED NOT NULL DEFAULT 0, KEY idx_admin_id (admin_id), KEY idx_target_admin_id (target_admin_id)) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + table + " (id, target_admin_id, admin_id) VALUES (1, 7, 3)").Error; err != nil {
		t.Fatal(err)
	}
	if err := applyCanonicalColumn(db, cfg, canonicalColumns[11]); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table(name).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("data count=%d err=%v", count, err)
	}
	for _, index := range []string{"idx_admin_id", "idx_target_admin_id"} {
		if !core.IndexExists(db, name, index) {
			t.Fatalf("index %s was lost", index)
		}
	}
	definition, ok, err := core.MigrationColumnInfo(db, name, "admin_id")
	if err != nil || !ok || definition.Ordinal != 2 || definition.Comment != "操作管理员" {
		t.Fatalf("admin_id=%#v ok=%v err=%v", definition, ok, err)
	}
}

func TestVersion0011RecoversExplicitTailOrderedSnapshot(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{Database: conf.Database{Prefix: fmt.Sprintf("c11_%d_", time.Now().UnixNano())}}
	models := map[string]any{
		"admin":                       &model.Admin{},
		"attachment":                  &model.Attachment{},
		"user":                        &model.User{},
		"user_money_log":              &model.UserMoneyLog{},
		"user_score_log":              &model.UserScoreLog{},
		"admin_log":                   &model.AdminLog{},
		"security_data_recycle":       &model.SecurityDataRecycle{},
		"security_sensitive_data":     &model.SecuritySensitiveData{},
		"crud_log":                    &model.CrudLog{},
		"security_data_recycle_log":   &model.SecurityDataRecycleLog{},
		"security_sensitive_data_log": &model.SecuritySensitiveDataLog{},
	}
	uniqueTables := map[string]bool{}
	tables := []string{}
	for _, spec := range canonicalColumns {
		if !uniqueTables[spec.table] {
			uniqueTables[spec.table] = true
			tables = append(tables, spec.table)
		}
	}
	for _, table := range tables {
		if err := db.Set("gorm:table_options", "ENGINE=InnoDB").Table(core.TableName(cfg, table)).AutoMigrate(models[table]); err != nil {
			t.Fatalf("create %s: %v", table, err)
		}
	}
	t.Cleanup(func() {
		for _, table := range tables {
			db.Exec("DROP TABLE IF EXISTS " + core.QuoteIdentifier(core.TableName(cfg, table)))
		}
	})
	lastColumns := map[string]string{}
	for _, table := range tables {
		var last string
		if err := db.Raw("SELECT column_name FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? ORDER BY ordinal_position DESC LIMIT 1", core.TableName(cfg, table)).Scan(&last).Error; err != nil {
			t.Fatal(err)
		}
		lastColumns[table] = last
	}
	for _, spec := range canonicalColumns {
		last := lastColumns[spec.table]
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(core.TableName(cfg, spec.table)) + " MODIFY COLUMN " + core.QuoteIdentifier(spec.column) + " " + spec.definition + " AFTER " + core.QuoteIdentifier(last)).Error; err != nil {
			t.Fatalf("tail-order %s.%s: %v", spec.table, spec.column, err)
		}
	}
	if err := version0011(db, cfg); err != nil {
		t.Fatal(err)
	}
	for _, spec := range canonicalColumns {
		definition, ok, err := core.MigrationColumnInfo(db, core.TableName(cfg, spec.table), spec.column)
		if err != nil || !ok || definition.Ordinal != spec.ordinal || !canonicalDefinitionSafe(definition, spec) {
			t.Fatalf("canonical %s.%s=%#v ok=%v err=%v", spec.table, spec.column, definition, ok, err)
		}
	}
}
