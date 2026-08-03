package migrations

import (
	"fmt"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations/internal/core"
	"buildadmin-go/internal/migrations/official"

	"gorm.io/gorm"
)

const installDataVersion = official.InstallDataVersion
const installDataName = official.InstallDataName

type InstallRecoveryState = official.InstallRecoveryState

const (
	InstallFresh         = official.InstallFresh
	InstallInterrupted   = official.InstallInterrupted
	InstallStrictUpgrade = official.InstallStrictUpgrade
)

func ValidatePrefix(config *conf.Configuration) error { return core.ValidatePrefix(config) }
func tableName(config *conf.Configuration, logicalName string) string {
	return core.TableName(config, logicalName)
}
func quoteIdentifier(value string) string       { return core.QuoteIdentifier(value) }
func tableExists(db *gorm.DB, name string) bool { return core.TableExists(db, name) }
func MarkSeedPending(db *gorm.DB, config *conf.Configuration) error {
	return official.MarkSeedPending(db, config)
}
func SeedPending(db *gorm.DB, config *conf.Configuration) (bool, error) {
	return official.SeedPending(db, config)
}
func MarkSeedCompleted(db *gorm.DB, config *conf.Configuration) error {
	return official.MarkSeedCompleted(db, config)
}
func DecideInstallRecovery(db *gorm.DB, config *conf.Configuration) (InstallRecoveryState, error) {
	return official.DecideInstallRecovery(db, config)
}

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
