package local

import (
	"fmt"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func version225(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	table := core.TableName(config, "attachment")
	if !core.TableExists(db, table) {
		return nil
	}
	if core.IndexExists(db, table, "idx_admin_id") {
		column, err := core.IndexFirstColumn(db, table, "idx_admin_id")
		if err != nil {
			return fmt.Errorf("inspect idx_admin_id on %s: %w", table, err)
		}
		if column != "admin_id" {
			return fmt.Errorf("idx_admin_id on %s starts with %q, want admin_id", table, column)
		}
		return nil
	}
	if err := db.Exec("CREATE INDEX `idx_admin_id` ON " + core.QuoteIdentifier(table) + " (`admin_id`)").Error; err != nil {
		return fmt.Errorf("add idx_admin_id to %s: %w", table, err)
	}
	return nil
}
