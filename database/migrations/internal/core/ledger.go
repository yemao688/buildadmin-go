package core

import (
	"go-build-admin/conf"
	"gorm.io/gorm"
)

func BootstrapLocalLedger(db *gorm.DB, config *conf.Configuration) error {
	return BootstrapTrackedLedger(db, config, "local_migrations", TrackedLedgerOptions{IncludeAdoptedFrom: true})
}

func ValidateLocalLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return ValidateTrackedLedgerSchema(db, config, "local_migrations", TrackedLedgerOptions{IncludeAdoptedFrom: true})
}

func InsertPendingLocalMigration(db *gorm.DB, config *conf.Configuration, m LocalMigration, adoptedFrom *string) error {
	return InsertPendingTrackedMigration(db, config, "local_migrations", TrackedMigration{Sequence: m.Sequence, ID: m.ID, Revision: m.Revision, Up: m.Up}, adoptedFrom)
}

func CompleteLocalMigration(db *gorm.DB, config *conf.Configuration, m LocalMigration) error {
	return CompleteTrackedMigration(db, config, "local_migrations", TrackedMigration{Sequence: m.Sequence, ID: m.ID, Revision: m.Revision}, "local")
}
