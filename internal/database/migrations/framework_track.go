package migrations

import (
	"go-build-admin/internal/conf"
	"go-build-admin/internal/database/migrations/framework"
	"go-build-admin/internal/database/migrations/internal/core"
	"gorm.io/gorm"
)

func FrameworkMigrations() []FrameworkMigration {
	return framework.Migrations(OfficialMigrations())
}

func EnsureAdminClosureSelfRows(db *gorm.DB, config *conf.Configuration) error {
	return framework.EnsureAdminClosureSelfRows(db, config)
}

func FrameworkVerifyCurrent(db *gorm.DB, config *conf.Configuration) error {
	return framework.VerifyCurrent(db, config)
}

type MigrationFn = core.MigrationFn
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
