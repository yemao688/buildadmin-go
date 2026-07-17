package local

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func version229(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := core.TableName(config, "admin")
	tables := []struct{ table, jsonColumn string }{
		{core.TableName(config, "security_data_recycle_log"), "data"},
		{core.TableName(config, "security_sensitive_data_log"), "before"},
	}
	for _, item := range tables {
		if err := addTargetOwnerColumnAndIndex(db, item.table); err != nil {
			return err
		}
		if core.TableExists(db, item.table) && core.ColumnExists(db, item.table, "target_admin_id") {
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
