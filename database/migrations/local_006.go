package migrations

import (
	"fmt"
	"go-build-admin/conf"
	"strings"

	"gorm.io/gorm"
)

func version228(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	for _, item := range []struct{ table, column string }{
		{tableName(config, "user_money_log"), "money"},
		{tableName(config, "user_score_log"), "score"},
	} {
		if !tableExists(db, item.table) {
			continue
		}
		def, ok, err := migrationColumnInfo(db, item.table, item.column)
		if err != nil {
			return err
		}
		if !ok || !strings.Contains(strings.ToLower(def.ColumnType), "unsigned") {
			continue
		}
		if err := db.Exec("ALTER TABLE " + quoteIdentifier(item.table) + " MODIFY COLUMN " + quoteIdentifier(item.column) + " int(11) NOT NULL DEFAULT 0").Error; err != nil {
			return fmt.Errorf("sign %s.%s: %w", item.table, item.column, err)
		}
	}
	return nil
}
