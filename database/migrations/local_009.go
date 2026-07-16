package migrations

import (
	"go-build-admin/conf"

	"gorm.io/gorm"
)

func version231(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	for _, table := range []string{tableName(config, "security_data_recycle_log"), tableName(config, "security_sensitive_data_log")} {
		if err := addCommittedColumn(db, table); err != nil {
			return err
		}
	}
	// Existing records intentionally remain is_committed=0. The migration must
	// not infer whether an old security operation completed successfully.
	return nil
}
