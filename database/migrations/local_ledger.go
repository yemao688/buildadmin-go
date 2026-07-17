package migrations

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

type MigrationFn = core.MigrationFn
type OfficialKey = core.OfficialKey
type OfficialMigration = core.OfficialMigration
type LocalMigration = core.LocalMigration
type LocalMigrationRecord = core.LocalMigrationRecord

func ValidateOfficialMigrations(list []OfficialMigration) error {
	return core.ValidateOfficialMigrations(list)
}

func ValidateLocalMigrations(list []LocalMigration, official []OfficialMigration) error {
	return core.ValidateLocalMigrations(list, official)
}

func BootstrapLocalLedger(db *gorm.DB, config *conf.Configuration) error {
	return core.BootstrapLocalLedger(db, config)
}

func ValidateLocalLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return core.ValidateLocalLedgerSchema(db, config)
}

func InsertPendingLocalMigration(db *gorm.DB, config *conf.Configuration, m LocalMigration, adoptedFrom *string) error {
	return core.InsertPendingLocalMigration(db, config, m, adoptedFrom)
}

func CompleteLocalMigration(db *gorm.DB, config *conf.Configuration, m LocalMigration) error {
	return core.CompleteLocalMigration(db, config, m)
}
