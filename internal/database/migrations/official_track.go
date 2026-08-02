package migrations

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/database/migrations/internal/core"
	"buildadmin-go/internal/database/migrations/official"
	"gorm.io/gorm"
)

func OfficialMigrations() []OfficialMigration {
	return official.Migrations()
}

func PrepareUpstreamNeutralSchema(db *gorm.DB, config *conf.Configuration) error {
	return official.PrepareUpstreamNeutralSchema(db, config)
}
func ReconcileLegacyData(db *gorm.DB, config *conf.Configuration) error {
	return official.ReconcileLegacyData(db, config)
}
func BootstrapOfficialLedger(db *gorm.DB, config *conf.Configuration) error {
	return core.BootstrapOfficialLedger(db, config)
}
func ValidateOfficialLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return core.ValidateOfficialLedgerSchema(db, config)
}
func RunOfficialMigrations(db *gorm.DB, config *conf.Configuration, list []OfficialMigration) (int, error) {
	return core.RunOfficialMigrations(db, config, list)
}
