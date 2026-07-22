package migrations

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/business"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

const businessLedgerName = "business_migrations"

func BusinessMigrations() ([]business.Migration, error) {
	return business.Migrations()
}

func BootstrapBusinessLedger(db *gorm.DB, config *conf.Configuration) error {
	return core.BootstrapTrackedLedger(db, config, businessLedgerName, core.TrackedLedgerOptions{})
}

func ValidateBusinessLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return core.ValidateTrackedLedgerSchema(db, config, businessLedgerName, core.TrackedLedgerOptions{})
}

func RunBusinessMigrations(db *gorm.DB, config *conf.Configuration, list []business.Migration) (int, error) {
	tracked := make([]core.TrackedMigration, 0, len(list))
	for _, migration := range list {
		tracked = append(tracked, core.TrackedMigration{Sequence: migration.Sequence, ID: migration.ID, Revision: migration.Revision, Up: migration.Up, VerifySchema: migration.VerifySchema, VerifyData: migration.VerifyData})
	}
	return core.RunTrackedMigrations(db, config, businessLedgerName, tracked, core.TrackedRunnerOptions{TrackName: "business"})
}
