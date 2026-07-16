package migrations

import (
	"fmt"
	"go-build-admin/conf"

	"gorm.io/gorm"
)

func version225(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	table := tableName(config, "attachment")
	if !tableExists(db, table) {
		return nil
	}
	if indexExists(db, table, "idx_admin_id") {
		column, err := indexFirstColumn(db, table, "idx_admin_id")
		if err != nil {
			return fmt.Errorf("inspect idx_admin_id on %s: %w", table, err)
		}
		if column != "admin_id" {
			return fmt.Errorf("idx_admin_id on %s starts with %q, want admin_id", table, column)
		}
		return nil
	}
	if err := db.Exec("CREATE INDEX `idx_admin_id` ON " + quoteIdentifier(table) + " (`admin_id`)").Error; err != nil {
		return fmt.Errorf("add idx_admin_id to %s: %w", table, err)
	}
	return nil
}

func version224(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := tableName(config, "admin")
	closureTable := tableName(config, "admin_closure")
	lockTable := tableName(config, "admin_hierarchy_lock")

	if tableExists(db, adminTable) {
		if !columnExists(db, adminTable, "parent_id") {
			if err := db.Exec(
				"ALTER TABLE " + quoteIdentifier(adminTable) +
					" ADD COLUMN `parent_id` int(11) unsigned DEFAULT NULL COMMENT '父级管理员ID'",
			).Error; err != nil {
				return fmt.Errorf("add parent_id to %s: %w", adminTable, err)
			}
		}
		if !indexExists(db, adminTable, "idx_parent_id") {
			if err := db.Exec(
				"CREATE INDEX `idx_parent_id` ON " + quoteIdentifier(adminTable) + " (`parent_id`)",
			).Error; err != nil {
				return fmt.Errorf("add idx_parent_id to %s: %w", adminTable, err)
			}
		}
	}

	if !tableExists(db, closureTable) {
		if err := execMigrationSQL(db, "local/0002-admin-hierarchy.sql", map[string]string{
			"closure": quoteIdentifier(closureTable),
			"lock":    quoteIdentifier(lockTable),
		}); err != nil {
			return fmt.Errorf("create %s: %w", closureTable, err)
		}
	}

	for _, idx := range []struct{ name, cols string }{
		{"idx_descendant_ancestor", "`descendant_id`,`ancestor_id`"},
		{"idx_ancestor_depth", "`ancestor_id`,`depth`"},
	} {
		if !indexExists(db, closureTable, idx.name) {
			if err := db.Exec(
				"CREATE INDEX " + quoteIdentifier(idx.name) + " ON " + quoteIdentifier(closureTable) + " (" + idx.cols + ")",
			).Error; err != nil {
				return fmt.Errorf("add %s to %s: %w", idx.name, closureTable, err)
			}
		}
	}

	if !tableExists(db, lockTable) {
		if err := execMigrationSQL(db, "local/0002-admin-hierarchy.sql", map[string]string{
			"closure": quoteIdentifier(closureTable),
			"lock":    quoteIdentifier(lockTable),
		}); err != nil {
			return fmt.Errorf("create %s: %w", lockTable, err)
		}
	}
	if err := db.Exec("INSERT IGNORE INTO " + quoteIdentifier(lockTable) + " (id) VALUES (1)").Error; err != nil {
		return fmt.Errorf("seed %s: %w", lockTable, err)
	}

	return EnsureAdminClosureSelfRows(db, config)
}
