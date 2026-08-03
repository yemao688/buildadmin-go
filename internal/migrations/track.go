package migrations

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations/framework"
	"buildadmin-go/internal/migrations/internal/core"
	"buildadmin-go/internal/migrations/official"
	"gorm.io/gorm"
)

// OfficialMigrations returns the official track registry (upstream identity).
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

func FrameworkMigrations() []FrameworkMigration {
	return framework.Migrations(OfficialMigrations())
}

func FrameworkVerifyCurrent(db *gorm.DB, config *conf.Configuration) error {
	return framework.VerifyCurrent(db, config)
}

type OfficialKey = core.OfficialKey
type OfficialMigration = core.OfficialMigration
type FrameworkMigration = core.FrameworkMigration
type FrameworkMigrationRecord = core.FrameworkMigrationRecord

func ValidateOfficialMigrations(list []OfficialMigration) error {
	return core.ValidateOfficialMigrations(list)
}

func ValidateFrameworkMigrations(list []FrameworkMigration, official []OfficialMigration) error {
	return core.ValidateFrameworkMigrations(list, official)
}

func BootstrapFrameworkLedger(db *gorm.DB, config *conf.Configuration) error {
	return core.BootstrapFrameworkLedger(db, config)
}

func ValidateFrameworkLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return core.ValidateFrameworkLedgerSchema(db, config)
}

func InsertPendingFrameworkMigration(db *gorm.DB, config *conf.Configuration, m FrameworkMigration) error {
	return core.InsertPendingFrameworkMigration(db, config, m)
}

func CompleteFrameworkMigration(db *gorm.DB, config *conf.Configuration, m FrameworkMigration) error {
	return core.CompleteFrameworkMigration(db, config, m)
}
