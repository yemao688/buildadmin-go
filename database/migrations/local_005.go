package migrations

import (
	"fmt"
	"go-build-admin/conf"

	"gorm.io/gorm"
)

func version227(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	tables := []string{
		tableName(config, "admin_log"),
		tableName(config, "security_data_recycle_log"),
		tableName(config, "security_sensitive_data_log"),
		tableName(config, "security_data_recycle"),
		tableName(config, "security_sensitive_data"),
		tableName(config, "crud_log"),
	}
	for _, table := range tables {
		if err := addAdminOwnerColumnAndIndex(db, table, "local/0005-security-ownership.sql"); err != nil {
			return err
		}
	}
	hasRows, err := migrationTablesHaveRows(db, tables)
	if err != nil {
		return err
	}
	adminTable := tableName(config, "admin")
	hasAdmin, err := legacyTableExists(db, adminTable)
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
		if tableExists(db, table) {
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
