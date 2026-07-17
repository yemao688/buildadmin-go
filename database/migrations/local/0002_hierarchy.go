package local

import (
	"fmt"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func version224(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := core.TableName(config, "admin")
	closureTable := core.TableName(config, "admin_closure")
	lockTable := core.TableName(config, "admin_hierarchy_lock")

	if core.TableExists(db, adminTable) {
		if !core.ColumnExists(db, adminTable, "parent_id") {
			if err := db.Exec(
				"ALTER TABLE " + core.QuoteIdentifier(adminTable) +
					" ADD COLUMN `parent_id` int(11) unsigned DEFAULT NULL COMMENT '父级管理员ID'",
			).Error; err != nil {
				return fmt.Errorf("add parent_id to %s: %w", adminTable, err)
			}
		}
		if !core.IndexExists(db, adminTable, "idx_parent_id") {
			if err := db.Exec(
				"CREATE INDEX `idx_parent_id` ON " + core.QuoteIdentifier(adminTable) + " (`parent_id`)",
			).Error; err != nil {
				return fmt.Errorf("add idx_parent_id to %s: %w", adminTable, err)
			}
		}
	}

	if !core.TableExists(db, closureTable) {
		if err := db.Exec("CREATE TABLE IF NOT EXISTS " + core.QuoteIdentifier(closureTable) + " (" +
			"`ancestor_id` int(11) unsigned NOT NULL," +
			"`descendant_id` int(11) unsigned NOT NULL," +
			"`depth` int(11) unsigned NOT NULL DEFAULT 0," +
			"PRIMARY KEY (`ancestor_id`,`descendant_id`)," +
			"KEY `idx_descendant_ancestor` (`descendant_id`,`ancestor_id`)," +
			"KEY `idx_ancestor_depth` (`ancestor_id`,`depth`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
			return fmt.Errorf("create %s: %w", closureTable, err)
		}
	}

	for _, idx := range []struct{ name, cols string }{
		{"idx_descendant_ancestor", "`descendant_id`,`ancestor_id`"},
		{"idx_ancestor_depth", "`ancestor_id`,`depth`"},
	} {
		if !core.IndexExists(db, closureTable, idx.name) {
			if err := db.Exec(
				"CREATE INDEX " + core.QuoteIdentifier(idx.name) + " ON " + core.QuoteIdentifier(closureTable) + " (" + idx.cols + ")",
			).Error; err != nil {
				return fmt.Errorf("add %s to %s: %w", idx.name, closureTable, err)
			}
		}
	}

	if !core.TableExists(db, lockTable) {
		if err := db.Exec("CREATE TABLE IF NOT EXISTS " + core.QuoteIdentifier(lockTable) + " (" +
			"`id` tinyint(3) unsigned NOT NULL," +
			"PRIMARY KEY (`id`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").Error; err != nil {
			return fmt.Errorf("create %s: %w", lockTable, err)
		}
	}
	if err := db.Exec("INSERT IGNORE INTO " + core.QuoteIdentifier(lockTable) + " (id) VALUES (1)").Error; err != nil {
		return fmt.Errorf("seed %s: %w", lockTable, err)
	}

	return EnsureAdminClosureSelfRows(db, config)
}
