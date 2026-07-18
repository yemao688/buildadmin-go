package local

import (
	"fmt"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

func version0012(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	for _, logical := range []string{"security_data_recycle", "security_sensitive_data"} {
		table := core.TableName(config, logical)
		if !core.TableExists(db, table) || core.ColumnExists(db, table, "owner_column") {
			continue
		}
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(table) + " ADD COLUMN `owner_column` varchar(64) NOT NULL DEFAULT 'admin_id' COMMENT '目标表所有者字段' AFTER `data_table`").Error; err != nil {
			return fmt.Errorf("add %s.owner_column: %w", table, err)
		}
	}
	return nil
}

func verifyOwnerColumnContract(db *gorm.DB, config *conf.Configuration) error {
	for _, logical := range []string{"security_data_recycle", "security_sensitive_data"} {
		table := core.TableName(config, logical)
		if !core.TableExists(db, table) {
			continue
		}
		if !core.ColumnExists(db, table, "owner_column") {
			return fmt.Errorf("%s.owner_column missing", table)
		}
	}
	return nil
}
