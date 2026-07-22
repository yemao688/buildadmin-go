package local

import (
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func version225(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	table := core.TableName(config, "attachment")
	if !core.TableExists(db, table) {
		return nil
	}
	if core.IndexExists(db, table, "idx_admin_id") {
		column, err := core.IndexFirstColumn(db, table, "idx_admin_id")
		if err != nil {
			return fmt.Errorf("inspect idx_admin_id on %s: %w", table, err)
		}
		if column != "admin_id" {
			return fmt.Errorf("idx_admin_id on %s starts with %q, want admin_id", table, column)
		}
		return nil
	}
	if err := db.Exec("CREATE INDEX `idx_admin_id` ON " + core.QuoteIdentifier(table) + " (`admin_id`)").Error; err != nil {
		return fmt.Errorf("add idx_admin_id to %s: %w", table, err)
	}
	return nil
}

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

func version227(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	tables := []string{core.TableName(config, "admin_log"), core.TableName(config, "security_data_recycle_log"), core.TableName(config, "security_sensitive_data_log"), core.TableName(config, "security_data_recycle"), core.TableName(config, "security_sensitive_data"), core.TableName(config, "crud_log")}
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

func version228(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	for _, item := range []struct{ table, column string }{{core.TableName(config, "user_money_log"), "money"}, {core.TableName(config, "user_score_log"), "score"}} {
		if !core.TableExists(db, item.table) {
			continue
		}
		def, ok, err := core.MigrationColumnInfo(db, item.table, item.column)
		if err != nil {
			return err
		}
		if !ok || !strings.Contains(strings.ToLower(def.ColumnType), "unsigned") {
			continue
		}
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(item.table) + " MODIFY COLUMN " + core.QuoteIdentifier(item.column) + " int(11) NOT NULL DEFAULT 0").Error; err != nil {
			return fmt.Errorf("sign %s.%s: %w", item.table, item.column, err)
		}
	}
	return nil
}

func version229(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	adminTable := core.TableName(config, "admin")
	for _, item := range []struct{ table, jsonColumn string }{{core.TableName(config, "security_data_recycle_log"), "data"}, {core.TableName(config, "security_sensitive_data_log"), "before"}} {
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

func version231(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	for _, table := range []string{core.TableName(config, "security_data_recycle_log"), core.TableName(config, "security_sensitive_data_log")} {
		if err := addCommittedColumn(db, table); err != nil {
			return err
		}
	}
	return nil
}

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

func normalizeFreshOwnership(db *gorm.DB, config *conf.Configuration) error {
	adminTable := core.TableName(config, "admin")
	if !core.TableExists(db, adminTable) {
		return nil
	}
	root, err := migrationRootID(db, config)
	if err != nil {
		return err
	}
	for _, logical := range []string{"user", "attachment", "admin_log", "security_data_recycle_log", "security_sensitive_data_log", "security_data_recycle", "security_sensitive_data", "crud_log"} {
		table := core.TableName(config, logical)
		if core.TableExists(db, table) && core.ColumnExists(db, table, "admin_id") {
			if err := repairMigrationOwners(db, table, adminTable, root); err != nil {
				return err
			}
		}
	}
	userTable := core.TableName(config, "user")
	for _, logical := range []string{"user_money_log", "user_score_log"} {
		table := core.TableName(config, logical)
		if core.TableExists(db, table) && core.TableExists(db, userTable) {
			if err := db.Exec("UPDATE " + core.QuoteIdentifier(table) + " l JOIN " + core.QuoteIdentifier(userTable) + " u ON u.id=l.user_id SET l.admin_id=u.admin_id").Error; err != nil {
				return err
			}
		}
	}
	return nil
}
