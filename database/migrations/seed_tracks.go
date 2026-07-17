package migrations

import (
	"fmt"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/local"
	"gorm.io/gorm"
)

// RunFreshSeed keeps the existing idempotent installer as the official seed
// baseline, then performs local invariant checks before completing InstallData.
// Install.InsertData owns its transaction; closure repair is deliberately kept
// outside it because it is an independent idempotent DDL/data boundary.
func RunFreshSeed(db *gorm.DB, config *conf.Configuration, locals []LocalMigration) error {
	db = db.Session(&gorm.Session{NewDB: true})
	return db.Transaction(func(tx *gorm.DB) error {
		if err := NewInstall(tx).InsertData(); err != nil {
			return fmt.Errorf("official seed baseline: %w", err)
		}
		if err := local.ApplyFreshOverlay(tx, config); err != nil {
			return fmt.Errorf("local seed overlay: %w", err)
		}
		for _, local := range locals {
			if local.PostSeedVerify != nil {
				if err := local.PostSeedVerify(tx, config); err != nil {
					return fmt.Errorf("post-seed verify %s: %w", local.ID, err)
				}
			}
		}
		return MarkSeedCompleted(tx, config)
	})
}
