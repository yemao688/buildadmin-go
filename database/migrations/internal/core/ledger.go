package core

import (
	"go-build-admin/conf"
	"gorm.io/gorm"
)

func BootstrapFrameworkLedger(db *gorm.DB, config *conf.Configuration) error {
	return BootstrapTrackedLedger(db, config, "migrations_framework")
}

func ValidateFrameworkLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return ValidateTrackedLedgerSchema(db, config, "migrations_framework")
}

func InsertPendingFrameworkMigration(db *gorm.DB, config *conf.Configuration, m FrameworkMigration) error {
	return InsertPendingTrackedMigration(db, config, "migrations_framework", TrackedMigration{Version: m.Version, MigrationName: m.MigrationName, Up: m.Up})
}

func CompleteFrameworkMigration(db *gorm.DB, config *conf.Configuration, m FrameworkMigration) error {
	return CompleteTrackedMigration(db, config, "migrations_framework", TrackedMigration{Version: m.Version, MigrationName: m.MigrationName}, "framework")
}
