package core

import (
	"go-build-admin/conf"
	"gorm.io/gorm"
)

func BootstrapFrameworkLedger(db *gorm.DB, config *conf.Configuration) error {
	return BootstrapTrackedLedger(db, config, "framework_migrations", TrackedLedgerOptions{})
}

func ValidateFrameworkLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return ValidateTrackedLedgerSchema(db, config, "framework_migrations", TrackedLedgerOptions{})
}

func InsertPendingFrameworkMigration(db *gorm.DB, config *conf.Configuration, m FrameworkMigration) error {
	return InsertPendingTrackedMigration(db, config, "framework_migrations", TrackedMigration{Sequence: m.Sequence, ID: m.ID, Revision: m.Revision, Up: m.Up})
}

func CompleteFrameworkMigration(db *gorm.DB, config *conf.Configuration, m FrameworkMigration) error {
	return CompleteTrackedMigration(db, config, "framework_migrations", TrackedMigration{Sequence: m.Sequence, ID: m.ID, Revision: m.Revision}, "framework")
}
