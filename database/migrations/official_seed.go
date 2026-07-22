package migrations

import (
	"fmt"
	"go-build-admin/conf"
	"go-build-admin/database/migrations/official"
	"gorm.io/gorm"
)

type Install = official.Install

func NewInstall(sqlDB *gorm.DB) *Install {
	return official.NewInstall(sqlDB)
}

func RunOfficialFreshSeed(db *gorm.DB, config *conf.Configuration) error {
	db = db.Session(&gorm.Session{NewDB: true})
	return db.Transaction(func(tx *gorm.DB) error {
		if err := NewInstall(tx).InsertData(); err != nil {
			return fmt.Errorf("official seed baseline: %w", err)
		}
		return MarkSeedCompleted(tx, config)
	})
}
