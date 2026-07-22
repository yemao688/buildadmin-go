package migrations

import (
	"fmt"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

func RunOfficialFreshSeed(db *gorm.DB, config *conf.Configuration) error {
	db = db.Session(&gorm.Session{NewDB: true})
	return db.Transaction(func(tx *gorm.DB) error {
		if err := NewInstall(tx).InsertData(); err != nil {
			return fmt.Errorf("official seed baseline: %w", err)
		}
		return MarkSeedCompleted(tx, config)
	})
}
