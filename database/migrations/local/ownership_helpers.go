package local

import (
	"fmt"
	"strings"

	"go-build-admin/app/pkg/systemroot"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func validOwnerColumn(def core.MigrationColumn, label string) error {
	typ := strings.ToLower(def.ColumnType)
	if !strings.Contains(typ, "int") || !strings.Contains(typ, "unsigned") || !strings.EqualFold(def.Nullable, "NO") || def.Default == nil || *def.Default != "0" {
		return fmt.Errorf("%s has invalid owner column schema", label)
	}
	return nil
}

func addAdminOwnerColumnAndIndex(db *gorm.DB, table string) error {
	exists, err := core.LegacyTableExists(db, table)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	def, ok, err := core.MigrationColumnInfo(db, table, "admin_id")
	if err != nil {
		return err
	}
	if !ok {
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(table) + " ADD COLUMN `admin_id` int(11) unsigned NOT NULL DEFAULT 0 COMMENT '管理员ID'").Error; err != nil {
			return err
		}
		def, ok, err = core.MigrationColumnInfo(db, table, "admin_id")
		if err != nil {
			return err
		}
	}
	if !ok {
		return fmt.Errorf("%s.admin_id was not created", table)
	}
	if err := validOwnerColumn(def, table+".admin_id"); err != nil {
		return err
	}
	has, first, err := core.MigrationIndexInfo(db, table, "idx_admin_id")
	if err != nil {
		return fmt.Errorf("inspect idx_admin_id on %s: %w", table, err)
	}
	if has {
		if first != "admin_id" {
			return fmt.Errorf("idx_admin_id on %s starts with %q, want admin_id", table, first)
		}
		return nil
	}
	return db.Exec("CREATE INDEX `idx_admin_id` ON " + core.QuoteIdentifier(table) + " (`admin_id`)").Error
}

func migrationRootID(db *gorm.DB, config *conf.Configuration) (int32, error) {
	adminTable := core.TableName(config, "admin")
	if ok, err := core.LegacyTableExists(db, adminTable); err != nil {
		return 0, err
	} else if !ok {
		return 0, fmt.Errorf("admin table %s does not exist", adminTable)
	}
	return (systemroot.Resolver{DB: db, AdminTable: adminTable}).Resolve()
}

func repairMigrationOwners(db *gorm.DB, table, adminTable string, rootID int32) error {
	return db.Exec("UPDATE "+core.QuoteIdentifier(table)+" t LEFT JOIN "+core.QuoteIdentifier(adminTable)+" a ON a.id=t.admin_id SET t.admin_id=? WHERE t.admin_id=0 OR t.admin_id IS NULL OR a.id IS NULL", rootID).Error
}

func validateMigrationOwners(db *gorm.DB, table, adminTable string) error {
	var invalid int64
	if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(table) + " t LEFT JOIN " + core.QuoteIdentifier(adminTable) + " a ON a.id=t.admin_id WHERE t.admin_id=0 OR a.id IS NULL").Scan(&invalid).Error; err != nil {
		return err
	}
	if invalid != 0 {
		return fmt.Errorf("%s contains %d invalid admin owner(s)", table, invalid)
	}
	return nil
}

func migrationTablesHaveRows(db *gorm.DB, tables []string) (bool, error) {
	for _, table := range tables {
		if !core.TableExists(db, table) {
			continue
		}
		var count int64
		if err := db.Table(table).Limit(1).Count(&count).Error; err != nil {
			return false, err
		}
		if count != 0 {
			return true, nil
		}
	}
	return false, nil
}

func validateLogOwnerMatchesUser(db *gorm.DB, logTable, userTable string) error {
	if !core.TableExists(db, logTable) || !core.TableExists(db, userTable) {
		return nil
	}
	var invalid int64
	if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(logTable) + " l JOIN " + core.QuoteIdentifier(userTable) + " u ON u.id=l.user_id WHERE l.admin_id<>u.admin_id").Scan(&invalid).Error; err != nil {
		return err
	}
	if invalid != 0 {
		return fmt.Errorf("%s contains %d owner mismatch(es)", logTable, invalid)
	}
	return nil
}
