package migrations

import (
	"fmt"
	"go-build-admin/conf"

	"gorm.io/gorm"
)

func version226(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	userTable := tableName(config, "user")
	logTables := []string{tableName(config, "user_money_log"), tableName(config, "user_score_log")}
	allTables := append([]string{userTable}, logTables...)
	for _, table := range allTables {
		if err := addAdminOwnerColumnAndIndex(db, table, "local/0004-user-ownership.sql"); err != nil {
			return err
		}
	}
	hasRows, err := migrationTablesHaveRows(db, allTables)
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
	if tableExists(db, userTable) {
		if err := repairMigrationOwners(db, userTable, adminTable, rootID); err != nil {
			return err
		}
		if err := validateMigrationOwners(db, userTable, adminTable); err != nil {
			return err
		}
	}
	for _, table := range logTables {
		if !tableExists(db, table) {
			continue
		}
		if tableExists(db, userTable) {
			if err := db.Exec("UPDATE " + quoteIdentifier(table) + " l JOIN " + quoteIdentifier(userTable) + " u ON u.id=l.user_id SET l.admin_id=u.admin_id").Error; err != nil {
				return err
			}
		}
		if err := db.Exec("UPDATE "+quoteIdentifier(table)+" l LEFT JOIN "+quoteIdentifier(adminTable)+" a ON a.id=l.admin_id SET l.admin_id=? WHERE l.admin_id=0 OR a.id IS NULL", rootID).Error; err != nil {
			return err
		}
		if err := validateMigrationOwners(db, table, adminTable); err != nil {
			return err
		}
		if err := validateLogOwnerMatchesUser(db, table, userTable); err != nil {
			return err
		}
	}
	return nil
}
