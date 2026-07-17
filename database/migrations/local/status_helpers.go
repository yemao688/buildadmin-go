package local

import (
	"fmt"
	"strings"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"

	"gorm.io/gorm"
)

var allowedAccountStatuses = map[string]string{"0": "disable", "1": "enable", "enable": "enable", "disable": "disable"}

func adminStatusNeedsBridge(db *gorm.DB, table string) (bool, error) {
	var columnType string
	result := db.Raw("SELECT column_type FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='status'", table).Scan(&columnType)
	if result.Error != nil {
		return false, result.Error
	}
	return strings.EqualFold(columnType, "enum('0','1')"), nil
}

func bridgeAdminStatusSchema(db *gorm.DB, config *conf.Configuration) error {
	t := core.TableName(config, "admin")
	if !core.TableExists(db, t) || !core.ColumnExists(db, t, "status") {
		return nil
	}
	needs, err := adminStatusNeedsBridge(db, t)
	if err != nil {
		return err
	}
	if needs {
		// Keep the old default until Version223 maps the data. If a later stage
		// fails, the legacy runtime can still create administrators safely.
		if err := db.Exec("ALTER TABLE " + core.QuoteIdentifier(t) + " MODIFY COLUMN `status` VARCHAR(30) NOT NULL DEFAULT '1'").Error; err != nil {
			return fmt.Errorf("bridge %s.status: %w", t, err)
		}
	}
	return nil
}

func mapAccountStatuses(values []string) ([]string, error) {
	result := make([]string, len(values))
	for i, value := range values {
		mapped, ok := allowedAccountStatuses[value]
		if !ok {
			return nil, fmt.Errorf("invalid account status %q", value)
		}
		result[i] = mapped
	}
	return result, nil
}

func accountStatusValues(db *gorm.DB, table string) ([]string, error) {
	var nullCount int64
	if err := db.Raw("SELECT COUNT(*) FROM " + core.QuoteIdentifier(table) + " WHERE status IS NULL").Scan(&nullCount).Error; err != nil {
		return nil, err
	}
	if nullCount > 0 {
		return nil, fmt.Errorf("null status")
	}
	var values []string
	if err := db.Raw("SELECT DISTINCT CAST(status AS BINARY) AS status FROM " + core.QuoteIdentifier(table)).Scan(&values).Error; err != nil {
		return nil, err
	}
	return values, nil
}

func alterAccountStatusColumn(db *gorm.DB, table string) error {
	return db.Exec("ALTER TABLE " + core.QuoteIdentifier(table) + " MODIFY COLUMN `status` VARCHAR(30) NOT NULL DEFAULT 'enable' COMMENT '状态:enable=启用,disable=禁用'").Error
}
