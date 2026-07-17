package local

import (
	"fmt"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func version226(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	userTable := core.TableName(config, "user")
	logTables := []string{core.TableName(config, "user_money_log"), core.TableName(config, "user_score_log")}
	allTables := append([]string{userTable}, logTables...)
	for _, table := range allTables {
		if err := addAdminOwnerColumnAndIndex(db, table); err != nil {
			return err
		}
	}
	hasRows, err := migrationTablesHaveRows(db, allTables)
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
	if core.TableExists(db, userTable) {
		if err := repairMigrationOwners(db, userTable, adminTable, rootID); err != nil {
			return err
		}
		if err := validateMigrationOwners(db, userTable, adminTable); err != nil {
			return err
		}
	}
	for _, table := range logTables {
		if !core.TableExists(db, table) {
			continue
		}
		if core.TableExists(db, userTable) {
			if err := db.Exec("UPDATE " + core.QuoteIdentifier(table) + " l JOIN " + core.QuoteIdentifier(userTable) + " u ON u.id=l.user_id SET l.admin_id=u.admin_id").Error; err != nil {
				return err
			}
		}
		if err := db.Exec("UPDATE "+core.QuoteIdentifier(table)+" l LEFT JOIN "+core.QuoteIdentifier(adminTable)+" a ON a.id=l.admin_id SET l.admin_id=? WHERE l.admin_id=0 OR a.id IS NULL", rootID).Error; err != nil {
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
