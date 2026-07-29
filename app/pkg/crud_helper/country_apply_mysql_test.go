package crud_helper

import (
	"go-build-admin/conf"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestCountrySpecsApplyWithoutAlterDrift(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	prefix := "ba_country_apply_test_"
	var databaseName string
	if err := db.Raw("SELECT DATABASE()").Scan(&databaseName).Error; err != nil {
		t.Fatal(err)
	}
	cfg := &conf.Configuration{Database: conf.Database{Database: databaseName, Prefix: prefix}}
	tables := []string{"country_language", "country_language_content", "country_currency", "crud_log"}
	for _, table := range tables {
		if err := db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`").Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, table := range tables {
			_ = db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`").Error
		}
	})

	statements := []string{
		"CREATE TABLE `" + prefix + "country_language` (`id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID', `lan` varchar(20) NOT NULL DEFAULT '' COMMENT '语言代码', `name` varchar(50) NOT NULL DEFAULT '' COMMENT '语言名称', `remark` varchar(255) NOT NULL DEFAULT '' COMMENT '备注', `status` tinyint unsigned NOT NULL DEFAULT 1 COMMENT '状态:0=禁用,1=启用', `weigh` int NOT NULL DEFAULT 0 COMMENT '权重', PRIMARY KEY (`id`), KEY `idx_country_language_status` (`status`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"CREATE TABLE `" + prefix + "country_language_content` (`id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID', `lan` varchar(20) NOT NULL DEFAULT '' COMMENT '语言代码', `group` varchar(50) NOT NULL DEFAULT '' COMMENT '分组', `key` varchar(100) NOT NULL DEFAULT '' COMMENT '键', `type` varchar(30) NOT NULL DEFAULT '' COMMENT '类型', `value` longtext COMMENT '值', PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"CREATE TABLE `" + prefix + "country_currency` (`id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID', `code` varchar(20) NOT NULL DEFAULT '' COMMENT '货币代码', `name` varchar(50) NOT NULL DEFAULT '' COMMENT '货币名称', `symbol` varchar(20) NOT NULL DEFAULT '' COMMENT '货币符号', `rate` decimal(20,8) NOT NULL DEFAULT 1 COMMENT '汇率', `status` tinyint unsigned NOT NULL DEFAULT 1 COMMENT '状态:0=禁用,1=启用', `weigh` int NOT NULL DEFAULT 0 COMMENT '权重', PRIMARY KEY (`id`), KEY `idx_country_currency_status` (`status`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"CREATE TABLE `" + prefix + "crud_log` (`id` int NOT NULL AUTO_INCREMENT, `admin_id` int NOT NULL, `table_name` varchar(200) NOT NULL, `table` blob, `fields` blob, `status` varchar(30) NOT NULL DEFAULT 'start', `comment` varchar(255), `connection` varchar(100) NOT NULL DEFAULT '', `sync` int NOT NULL DEFAULT 0, `create_time` bigint NOT NULL, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"CREATE UNIQUE INDEX `uk_country_language_lan` ON `" + prefix + "country_language` (`lan`)",
		"CREATE UNIQUE INDEX `uk_country_language_content_lan_group_key` ON `" + prefix + "country_language_content` (`lan`, `group`, `key`)",
		"CREATE UNIQUE INDEX `uk_country_currency_code` ON `" + prefix + "country_currency` (`code`)",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	specDir := filepath.Join(utils.RootPath(), "crud_specs")
	paths := []string{
		filepath.Join(specDir, "country_currency.yaml"),
		filepath.Join(specDir, "country_language.yaml"),
		filepath.Join(specDir, "country_language_content.yaml"),
	}
	results, err := ApplySpecs(db, cfg, paths, ApplyOptions{AdminID: 1, SkipMenu: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(paths) {
		t.Fatalf("results = %d, want %d", len(results), len(paths))
	}
	for _, result := range results {
		if result.Action != ApplyUnchanged || len(result.Changes) != 0 {
			t.Errorf("%s: action=%s changes=%v, want unchanged with no changes", result.Table, result.Action, result.Changes)
		}
	}

	var drift int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name LIKE ? AND column_name = 'type' AND column_type <> 'varchar(30)'", prefix+"%").Scan(&drift).Error; err != nil {
		t.Fatal(err)
	}
	if drift != 0 {
		t.Fatalf("country language content type drift count = %d", drift)
	}
}
