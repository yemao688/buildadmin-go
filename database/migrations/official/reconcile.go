package official

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

func ReconcileLegacyData(db *gorm.DB, config *conf.Configuration) error {
	if err := version200(db, config); err != nil {
		return err
	}
	if err := version202(db, config); err != nil {
		return err
	}
	return db.Table(core.TableName(config, "security_data_recycle")).Where("data_table = ? AND controller_as = ?", "menu_rule", "auth/menu").Updates(map[string]any{"data_table": "admin_rule", "controller_as": "auth/rule"}).Error
}
