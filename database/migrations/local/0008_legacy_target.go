package local

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func version230(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := core.TableName(config, "admin")
	for _, table := range []string{core.TableName(config, "security_data_recycle_log"), core.TableName(config, "security_sensitive_data_log")} {
		if err := addLegacyTargetFlagColumn(db, table); err != nil {
			return err
		}
		if !core.TableExists(db, table) {
			continue
		}
		if err := db.Table(table).Where("target_admin_id=0").Update("legacy_unrecoverable", 1).Error; err != nil {
			return err
		}
		if err := validateTargetOwners(db, table, adminTable); err != nil {
			return err
		}
	}
	return nil
}
