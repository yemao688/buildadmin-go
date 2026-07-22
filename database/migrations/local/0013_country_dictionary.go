package local

import (
	"fmt"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
	"strings"
)

func version0013(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	statements := []string{
		"CREATE TABLE IF NOT EXISTS " + core.QuoteIdentifier(core.TableName(config, "country_language")) + " (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `lan` varchar(20) NOT NULL DEFAULT '', `name` varchar(50) NOT NULL DEFAULT '', `remark` varchar(255) NOT NULL DEFAULT '', `status` tinyint unsigned NOT NULL DEFAULT 1, `weigh` int NOT NULL DEFAULT 0, PRIMARY KEY (`id`), KEY `idx_country_language_status` (`status`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"CREATE TABLE IF NOT EXISTS " + core.QuoteIdentifier(core.TableName(config, "country_language_content")) + " (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `lan` varchar(20) NOT NULL DEFAULT '', `group` varchar(50) NOT NULL DEFAULT '', `key` varchar(100) NOT NULL DEFAULT '', `type` varchar(30) NOT NULL DEFAULT '', `value` longtext, PRIMARY KEY (`id`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"CREATE TABLE IF NOT EXISTS " + core.QuoteIdentifier(core.TableName(config, "country_currency")) + " (`id` bigint unsigned NOT NULL AUTO_INCREMENT, `code` varchar(20) NOT NULL DEFAULT '', `name` varchar(50) NOT NULL DEFAULT '', `symbol` varchar(20) NOT NULL DEFAULT '', `rate` decimal(20,8) NOT NULL DEFAULT 1, `status` tinyint unsigned NOT NULL DEFAULT 1, `weigh` int NOT NULL DEFAULT 0, PRIMARY KEY (`id`), KEY `idx_country_currency_status` (`status`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("create country dictionary table: %w", err)
		}
	}
	for _, column := range []struct{ table, name, definition string }{
		{"country_language", "remark", "varchar(255) NOT NULL DEFAULT '' COMMENT '备注'"},
		{"country_language_content", "type", "varchar(30) NOT NULL DEFAULT '' COMMENT '类型'"},
		{"country_currency", "rate", "decimal(20,8) NOT NULL DEFAULT 1 COMMENT '汇率'"},
	} {
		table := core.TableName(config, column.table)
		if !core.ColumnExists(db, table, column.name) {
			if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(table) + " ADD COLUMN " + core.QuoteIdentifier(column.name) + " " + column.definition).Error; err != nil {
				return fmt.Errorf("add %s.%s: %w", table, column.name, err)
			}
		}
	}
	content := core.TableName(config, "country_language_content")
	indexes := []struct {
		table   string
		name    string
		columns []string
	}{
		{core.TableName(config, "country_language"), "uk_country_language_lan", []string{"lan"}},
		{content, "uk_country_language_content_lan_group_key", []string{"lan", "group", "key"}},
		{core.TableName(config, "country_currency"), "uk_country_currency_code", []string{"code"}},
	}
	for _, index := range indexes {
		has, first, err := core.MigrationIndexInfo(db, index.table, index.name)
		if err != nil {
			return err
		}
		if has {
			if first != index.columns[0] {
				return fmt.Errorf("%s.%s starts with %q", index.table, index.name, first)
			}
			continue
		}
		quotedColumns := make([]string, len(index.columns))
		for i, column := range index.columns {
			quotedColumns[i] = core.QuoteIdentifier(column)
		}
		if err := db.Exec("CREATE UNIQUE INDEX " + core.QuoteIdentifier(index.name) + " ON " + core.QuoteIdentifier(index.table) + " (" + strings.Join(quotedColumns, ", ") + ")").Error; err != nil {
			return fmt.Errorf("create country dictionary unique index %s: %w", index.name, err)
		}
	}
	return seedCountryMenus(db, config)
}

// seedCountryMenus 为 country 字典模块种下后台菜单(幂等,按 name 判重):
// 国家管理(country 目录)下平级三个菜单——货币管理、语言管理、语言文本管理,
// 菜单 name 与提交的生成代码位置一致(country/languageContent 为单级)。
func seedCountryMenus(db *gorm.DB, config *conf.Configuration) error {
	table := core.QuoteIdentifier(core.TableName(config, "admin_rule"))

	ruleID := func(name string) (int32, error) {
		var id int32
		err := db.Raw("SELECT id FROM "+table+" WHERE name = ?", name).Scan(&id).Error
		return id, err
	}
	ensure := func(pid int32, ruleType, title, name, path, menuType, component string, weigh int) error {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM "+table+" WHERE name = ?", name).Scan(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		return db.Exec("INSERT INTO "+table+" (pid, type, title, name, path, menu_type, component, weigh, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '1')",
			pid, ruleType, title, name, path, menuType, component, weigh).Error
	}

	if err := ensure(0, "menu_dir", "国家管理", "country", "country", "", "", 0); err != nil {
		return fmt.Errorf("seed country menu dir: %w", err)
	}
	countryID, err := ruleID("country")
	if err != nil {
		return err
	}

	// weigh desc 排序:货币管理 > 语言管理 > 语言文本管理
	menus := []struct {
		title, name string
		weigh       int
	}{
		{"货币管理", "country/currency", 3},
		{"语言管理", "country/language", 2},
		{"语言文本管理", "country/languageContent", 1},
	}
	buttons := []struct{ title, suffix string }{
		{"查看", "/index"}, {"添加", "/add"}, {"编辑", "/edit"}, {"删除", "/del"}, {"快速排序", "/sortable"},
	}
	for _, menu := range menus {
		if err := ensure(countryID, "menu", menu.title, menu.name, menu.name, "tab", "/src/views/backend/"+menu.name+"/index.vue", menu.weigh); err != nil {
			return fmt.Errorf("seed country menu %s: %w", menu.name, err)
		}
		menuID, err := ruleID(menu.name)
		if err != nil {
			return err
		}
		for _, button := range buttons {
			if err := ensure(menuID, "button", button.title, menu.name+button.suffix, "", "", "", 0); err != nil {
				return fmt.Errorf("seed country menu button %s%s: %w", menu.name, button.suffix, err)
			}
		}
	}
	return nil
}

func verifyCountryDictionaryContract(db *gorm.DB, config *conf.Configuration) error {
	for _, table := range []string{"country_language", "country_language_content", "country_currency"} {
		if err := requireTable(db, core.TableName(config, table)); err != nil {
			return err
		}
	}
	for _, item := range []struct{ table, column string }{
		{"country_language", "lan"}, {"country_language", "name"}, {"country_language", "remark"}, {"country_language", "status"}, {"country_language", "weigh"},
		{"country_language_content", "lan"}, {"country_language_content", "group"}, {"country_language_content", "key"}, {"country_language_content", "type"}, {"country_language_content", "value"},
		{"country_currency", "code"}, {"country_currency", "name"}, {"country_currency", "symbol"}, {"country_currency", "rate"}, {"country_currency", "status"}, {"country_currency", "weigh"},
	} {
		if err := requireColumn(db, core.TableName(config, item.table), item.column); err != nil {
			return err
		}
	}
	if err := requireIndexColumns(db, core.TableName(config, "country_language_content"), "uk_country_language_content_lan_group_key", []string{"lan", "group", "key"}); err != nil {
		return err
	}
	if err := requireIndexColumns(db, core.TableName(config, "country_language"), "uk_country_language_lan", []string{"lan"}); err != nil {
		return err
	}
	if err := requireIndexColumns(db, core.TableName(config, "country_currency"), "uk_country_currency_code", []string{"code"}); err != nil {
		return err
	}
	for _, table := range []string{"country_language", "country_currency"} {
		var invalid int64
		if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(core.TableName(config, table)) + " WHERE status NOT IN (0, 1) OR status IS NULL").Scan(&invalid).Error; err != nil {
			return err
		}
		if invalid != 0 {
			return fmt.Errorf("%s.status contains invalid values", table)
		}
	}
	return nil
}
