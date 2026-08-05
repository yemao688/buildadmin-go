package crud_helper

import (
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/pkg/util"
	"path/filepath"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestCountrySpecsApplyWithoutAlterDrift(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	prefix := "ba_country_apply_test_"
	cfg.Database.Prefix = prefix
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
		"CREATE TABLE `" + prefix + "country_language_content` (`id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'ID', `lan` varchar(20) NOT NULL DEFAULT '' COMMENT '语言代码', `group` varchar(50) NOT NULL DEFAULT '' COMMENT '分组', `key` varchar(100) NOT NULL DEFAULT '' COMMENT '键', `type` varchar(30) NOT NULL DEFAULT '' COMMENT '类型:0=文本,1=富文本,2=图片', `value` longtext COMMENT '值', PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
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

	specDir := filepath.Join(util.RootPath(), "crud_specs")
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

// TestCountryFreshInstallApplyUnblocked 模拟全新库路径：core model 的 gorm tag
// 驱动 AutoMigrate 快照建表（与 orchestrator 全新安装一致），随后尾部 apply
// 三张 country spec。此前 model 缺 default tag（spec 声明 EMPTY STRING/INPUT
// default），AutoMigrate 建出的列 default 为 NULL → apply 判 requires-approval
// blocked，全新安装被框架自带 spec 阻塞（下游 issue）。model 补齐 default 后
// 本测试应全部 unchanged。
func TestCountryFreshInstallApplyUnblocked(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	cfg.Database.Prefix = "ba_country_fresh_"
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: cfg.Database.Prefix}
	t.Cleanup(func() {
		for _, table := range []string{"country_language", "country_language_content", "country_currency", "crud_log"} {
			_ = db.Exec("DROP TABLE IF EXISTS `" + cfg.Database.Prefix + table + "`").Error
		}
	})

	migrateDB := db.Session(&gorm.Session{NewDB: true})
	if err := migrateDB.Set("gorm:table_options", "ENGINE=InnoDB").AutoMigrate(
		&model.CountryLanguage{}, &model.CountryLanguageContent{}, &model.CountryCurrency{}, &model.Log{},
	); err != nil {
		t.Fatal(err)
	}

	specDir := filepath.Join(util.RootPath(), "crud_specs")
	paths := []string{
		filepath.Join(specDir, "country_language.yaml"),
		filepath.Join(specDir, "country_language_content.yaml"),
		filepath.Join(specDir, "country_currency.yaml"),
	}
	results, err := ApplySpecs(db, cfg, paths, ApplyOptions{AdminID: 1, SkipMenu: true})
	if err != nil {
		t.Fatalf("fresh-install tail apply blocked: %v", err)
	}
	for _, result := range results {
		if result.Action != ApplyUnchanged {
			t.Errorf("%s: action=%s, want unchanged (fresh install must not be blocked)", result.Table, result.Action)
		}
	}
}
