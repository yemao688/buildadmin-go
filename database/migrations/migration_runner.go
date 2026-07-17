package migrations

import (
	"database/sql"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

func validateMigrationLockRelease(released sql.NullInt64) error {
	return core.ValidateMigrationLockRelease(released)
}

func WithMigrationLock(db *gorm.DB, name string, timeout time.Duration, fn func(*gorm.DB) error) error {
	return core.WithMigrationLock(db, name, timeout, fn)
}

func RunLocalMigrations(db *gorm.DB, config *conf.Configuration, official []OfficialMigration, local []LocalMigration) (int, error) {
	return core.RunLocalMigrations(db, config, official, local)
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
