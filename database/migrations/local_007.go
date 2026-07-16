package migrations

import (
	"go-build-admin/conf"

	"gorm.io/gorm"
)

func version229(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := tableName(config, "admin")
	tables := []struct{ table, jsonColumn string }{
		{tableName(config, "security_data_recycle_log"), "data"},
		{tableName(config, "security_sensitive_data_log"), "before"},
	}
	for _, item := range tables {
		if err := addTargetOwnerColumnAndIndex(db, item.table); err != nil {
			return err
		}
		if tableExists(db, item.table) && columnExists(db, item.table, "target_admin_id") {
			if err := backfillTargetOwnerFromJSON(db, item.table, item.jsonColumn, adminTable); err != nil {
				return err
			}
			if err := validateTargetOwners(db, item.table, adminTable); err != nil {
				return err
			}
		}
	}
	return nil
}
