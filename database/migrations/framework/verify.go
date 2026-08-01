package framework

import (
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func verifyFinalTableContract(db *gorm.DB, config *conf.Configuration) error {
	return verifyFinalTableContractImpl(db, config)
}

func verifyFinalDataContract(db *gorm.DB, config *conf.Configuration) error {
	return verifyFinalDataContractImpl(db, config)
}

// VerifyCurrent validates cross-table invariants after framework and business migrations.
func VerifyCurrent(db *gorm.DB, config *conf.Configuration) error {
	if err := validateMigrationOwners(db, core.TableName(config, "user"), core.TableName(config, "admin")); err != nil {
		return err
	}
	if err := validateClosureSelfRows(db, config); err != nil {
		return err
	}
	return verifySecuritySeedIdentity(db, config)
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

func verifySecuritySeedIdentity(db *gorm.DB, config *conf.Configuration) error {
	for _, check := range []struct {
		table, id, name, controllerAs, dataTable string
	}{
		{"security_data_recycle", "5", "会员", "user/user", "user"},
		{"security_sensitive_data", "2", "会员数据", "user/user", "user"},
	} {
		table := core.TableName(config, check.table)
		if err := requireTable(db, table); err != nil {
			return err
		}
		var row struct{ Name, ControllerAs, DataTable string }
		if err := db.Table(table).Where("id = ?", check.id).First(&row).Error; err != nil {
			return err
		}
		var duplicates int64
		if err := db.Table(table).Where("name=? AND controller=? AND controller_as=? AND data_table=? AND primary_key=?", check.name, "user.User", check.controllerAs, check.dataTable, "id").Count(&duplicates).Error; err != nil {
			return err
		}
		if duplicates != 1 {
			return fmt.Errorf("%s final installer identity count=%d", table, duplicates)
		}
		if row.Name != check.name || row.ControllerAs != check.controllerAs || row.DataTable != check.dataTable {
			return fmt.Errorf("%s seed %s has unexpected identity", table, check.id)
		}
		if check.table == "security_sensitive_data" {
			var fields string
			if err := db.Table(table).Where("id=?", check.id).Pluck("data_fields", &fields).Error; err != nil {
				return err
			}
			if strings.Contains(fields, "password") {
				return fmt.Errorf("%s final seed still exposes password", table)
			}
		}
	}
	return nil
}

func requireTable(db *gorm.DB, table string) error {
	ok, err := core.LegacyTableExists(db, table)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("required migration table %s is missing", table)
	}
	return nil
}

func requireColumn(db *gorm.DB, table, column string) error {
	ok, err := core.LegacyColumnExists(db, table, column)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("required migration column %s.%s is missing", table, column)
	}
	return nil
}

func requireIndexColumns(db *gorm.DB, table, index string, want []string) error {
	var rows []string
	if err := db.Raw("SELECT column_name FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? ORDER BY seq_in_index", table, index).Scan(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(want) {
		return fmt.Errorf("required migration index %s.%s has wrong columns", table, index)
	}
	for i := range want {
		if rows[i] != want[i] {
			return fmt.Errorf("required migration index %s.%s has wrong columns", table, index)
		}
	}
	return nil
}
