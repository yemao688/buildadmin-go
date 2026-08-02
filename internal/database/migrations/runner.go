package migrations

import (
	"database/sql"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/database/migrations/internal/core"
	"gorm.io/gorm"
)

func validateMigrationLockRelease(released sql.NullInt64) error {
	return core.ValidateMigrationLockRelease(released)
}

func WithMigrationLock(db *gorm.DB, name string, timeout time.Duration, fn func(*gorm.DB) error) error {
	return core.WithMigrationLock(db, name, timeout, fn)
}

func RunFrameworkMigrations(db *gorm.DB, config *conf.Configuration, official []OfficialMigration, framework []FrameworkMigration) (int, error) {
	return core.RunFrameworkMigrations(db, config, official, framework)
}
