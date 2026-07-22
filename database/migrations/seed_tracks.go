package migrations

import (
	"fmt"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/local"
	"gorm.io/gorm"
)

func RunOfficialFreshSeed(db *gorm.DB, config *conf.Configuration) error {
	db = db.Session(&gorm.Session{NewDB: true})
	return db.Transaction(func(tx *gorm.DB) error {
		if err := NewInstall(tx).InsertData(); err != nil {
			return fmt.Errorf("official seed baseline: %w", err)
		}
		if err := local.ApplyFreshOverlay(tx, config); err != nil {
			return fmt.Errorf("local seed overlay: %w", err)
		}
		return MarkSeedCompleted(tx, config)
	})
}

func RunPostSeedVerify(db *gorm.DB, config *conf.Configuration, locals []LocalMigration) error {
	for _, migration := range locals {
		if migration.PostSeedVerify == nil {
			continue
		}
		if err := migration.PostSeedVerify(db, config); err != nil {
			return fmt.Errorf("post-seed verify %s: %w", migration.ID, err)
		}
	}
	return nil
}
