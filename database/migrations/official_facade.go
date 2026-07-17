package migrations

import (
	"go-build-admin/conf"
	"go-build-admin/database/migrations/official"
	"gorm.io/gorm"
)

func PrepareUpstreamNeutralSchema(db *gorm.DB, config *conf.Configuration) error {
	return official.PrepareUpstreamNeutralSchema(db, config)
}

func ReconcileLegacyData(db *gorm.DB, config *conf.Configuration) error {
	return official.ReconcileLegacyData(db, config)
}
