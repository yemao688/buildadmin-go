package local

import (
	"fmt"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func EnsureAdminClosureSelfRows(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := core.TableName(config, "admin")
	closureTable := core.TableName(config, "admin_closure")
	if !core.TableExists(db, adminTable) {
		return nil
	}
	if !core.TableExists(db, closureTable) {
		return fmt.Errorf("%s does not exist", closureTable)
	}
	if err := db.Exec(
		"INSERT IGNORE INTO " + core.QuoteIdentifier(closureTable) + " (ancestor_id, descendant_id, depth) " +
			"SELECT id, id, 0 FROM " + core.QuoteIdentifier(adminTable),
	).Error; err != nil {
		return fmt.Errorf("backfill %s self rows: %w", closureTable, err)
	}
	return nil
}

func validateClosureSelfRows(db *gorm.DB, config *conf.Configuration) error {
	closure, admin := core.TableName(config, "admin_closure"), core.TableName(config, "admin")
	if !core.TableExists(db, closure) || !core.TableExists(db, admin) {
		return nil
	}
	var missing int64
	if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(admin) + " a LEFT JOIN " + core.QuoteIdentifier(closure) + " c ON c.ancestor_id=a.id AND c.descendant_id=a.id AND c.depth=0 WHERE c.ancestor_id IS NULL").Scan(&missing).Error; err != nil {
		return err
	}
	if missing != 0 {
		return fmt.Errorf("admin closure missing %d self row(s)", missing)
	}
	return nil
}
