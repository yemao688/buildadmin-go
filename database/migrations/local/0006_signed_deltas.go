package local

import (
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func version228(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	for _, item := range []struct{ table, column string }{
		{core.TableName(config, "user_money_log"), "money"},
		{core.TableName(config, "user_score_log"), "score"},
	} {
		if !core.TableExists(db, item.table) {
			continue
		}
		def, ok, err := core.MigrationColumnInfo(db, item.table, item.column)
		if err != nil {
			return err
		}
		if !ok || !strings.Contains(strings.ToLower(def.ColumnType), "unsigned") {
			continue
		}
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(item.table) + " MODIFY COLUMN " + core.QuoteIdentifier(item.column) + " int(11) NOT NULL DEFAULT 0").Error; err != nil {
			return fmt.Errorf("sign %s.%s: %w", item.table, item.column, err)
		}
	}
	return nil
}
