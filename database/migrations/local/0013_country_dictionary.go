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
