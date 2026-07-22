package migrations

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/local"
	"gorm.io/gorm"
)

func LocalMigrations() []LocalMigration {
	return local.Migrations(OfficialMigrations())
}

func EnsureAdminClosureSelfRows(db *gorm.DB, config *conf.Configuration) error {
	return local.EnsureAdminClosureSelfRows(db, config)
}

func LocalVerifyCurrent(db *gorm.DB, config *conf.Configuration) error {
	return local.VerifyCurrent(db, config)
}
