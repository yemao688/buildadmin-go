package local

import (
	"fmt"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

func local0001Up(db *gorm.DB, config *conf.Configuration) error {
	if err := bridgeAdminStatusSchema(db, config); err != nil {
		return err
	}
	return version223(db, config)
}

func version223(db *gorm.DB, config *conf.Configuration) error {
	tables := []string{core.TableName(config, "admin"), core.TableName(config, "user")}
	values := make([][]string, len(tables))
	present := make([]bool, len(tables))
	for i, table := range tables {
		if !core.TableExists(db, table) || !core.ColumnExists(db, table, "status") {
			continue
		}
		present[i] = true
		var err error
		values[i], err = accountStatusValues(db, table)
		if err != nil {
			return fmt.Errorf("preflight %s.status: %w", table, err)
		}
		if _, err := mapAccountStatuses(values[i]); err != nil {
			return fmt.Errorf("preflight %s.status: %w", table, err)
		}
	}
	for i, table := range tables {
		if !present[i] {
			continue
		}
		if err := alterAccountStatusColumn(db, table); err != nil {
			return fmt.Errorf("alter %s.status: %w", table, err)
		}
		if err := db.Table(table).Where("status = ?", "0").Update("status", "disable").Error; err != nil {
			return err
		}
		if err := db.Table(table).Where("status = ?", "1").Update("status", "enable").Error; err != nil {
			return err
		}
		var invalid int64
		if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(table) + " WHERE status IS NULL OR BINARY status NOT IN ('enable', 'disable')").Scan(&invalid).Error; err != nil {
			return err
		}
		if invalid != 0 {
			return fmt.Errorf("%s.status contains invalid values after migration", table)
		}
	}
	return nil
}
