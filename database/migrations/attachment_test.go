package migrations

import (
	"os"
	"testing"

	"go-build-admin/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestVersion225AttachmentIndexCustomPrefixAndRetry(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	prefix := "mig225_rt_"
	table := "`" + prefix + "attachment`"
	db.Exec("DROP TABLE IF EXISTS " + table)
	defer db.Exec("DROP TABLE IF EXISTS " + table)
	if err := db.Exec("CREATE TABLE " + table + " (id INT PRIMARY KEY, admin_id INT NOT NULL, label VARCHAR(20) NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	if err := version225(db, cfg); err != nil {
		t.Fatal(err)
	}
	if err := version225(db, cfg); err != nil {
		t.Fatal("retry should be idempotent: ", err)
	}
	var column string
	if err := db.Raw("SELECT column_name FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name='idx_admin_id' AND seq_in_index=1", prefix+"attachment").Scan(&column).Error; err != nil {
		t.Fatal(err)
	}
	if column != "admin_id" {
		t.Fatalf("idx_admin_id first column = %q", column)
	}
}

func TestVersion225RejectsWrongNamedIndex(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	prefix := "mig225_bad_"
	table := "`" + prefix + "attachment`"
	db.Exec("DROP TABLE IF EXISTS " + table)
	defer db.Exec("DROP TABLE IF EXISTS " + table)
	if err := db.Exec("CREATE TABLE " + table + " (id INT PRIMARY KEY, admin_id INT NOT NULL, label VARCHAR(20) NOT NULL, KEY idx_admin_id (label))").Error; err != nil {
		t.Fatal(err)
	}
	if err := version225(db, &conf.Configuration{Database: conf.Database{Prefix: prefix}}); err == nil {
		t.Fatal("expected wrong named index to fail")
	}
}
