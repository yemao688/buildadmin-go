package local

import (
	"fmt"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func version227(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	tables := []string{
		core.TableName(config, "admin_log"),
		core.TableName(config, "security_data_recycle_log"),
		core.TableName(config, "security_sensitive_data_log"),
		core.TableName(config, "security_data_recycle"),
		core.TableName(config, "security_sensitive_data"),
		core.TableName(config, "crud_log"),
	}
	for _, table := range tables {
		if err := addAdminOwnerColumnAndIndex(db, table); err != nil {
			return err
		}
	}
	hasRows, err := migrationTablesHaveRows(db, tables)
	if err != nil {
		return err
	}
	adminTable := core.TableName(config, "admin")
	hasAdmin, err := core.LegacyTableExists(db, adminTable)
	if err != nil {
		return err
	}
	if !hasRows {
		return nil
	}
	if !hasAdmin {
		return fmt.Errorf("admin table %s does not exist while ownership backfill is required", adminTable)
	}
	rootID, err := migrationRootID(db, config)
	if err != nil {
		return err
	}
	for _, table := range tables {
		if core.TableExists(db, table) {
			if err := repairMigrationOwners(db, table, adminTable, rootID); err != nil {
				return err
			}
			if err := validateMigrationOwners(db, table, adminTable); err != nil {
				return err
			}
		}
	}
	return nil
}
