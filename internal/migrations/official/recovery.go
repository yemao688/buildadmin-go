package official

import (
	"errors"
	"fmt"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/migrations/internal/core"
	"gorm.io/gorm"
)

type InstallRecoveryState string

const (
	InstallFresh         InstallRecoveryState = "fresh"
	InstallInterrupted   InstallRecoveryState = "interrupted_install"
	InstallStrictUpgrade InstallRecoveryState = "strict_upgrade"
)

const (
	InstallDataVersion int64 = 20230620180916
	InstallDataName          = "InstallData"
)

func DecideInstallRecovery(db *gorm.DB, config *conf.Configuration) (InstallRecoveryState, error) {
	if err := core.ValidatePrefix(config); err != nil {
		return "", err
	}
	ledgerExists, err := core.LegacyTableExists(db, core.TableName(config, "migrations"))
	if err != nil {
		return "", err
	}
	markerPending := false
	markerFound := false
	if ledgerExists {
		var marker core.MigrationRecord
		result := db.Table(core.TableName(config, "migrations")).Where("version = ?", InstallDataVersion).First(&marker)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return "", result.Error
		}
		if result.Error == nil {
			markerFound = true
			if marker.MigrationName != InstallDataName {
				return "", fmt.Errorf("migration version %d name collision", InstallDataVersion)
			}
			markerPending = marker.EndTime == nil
		}
	}
	businessExists := false
	for _, name := range core.CoreLogicalNames() {
		if name == "migrations" {
			// The official ledger tracks installation state; it is not business schema.
			continue
		}
		ok, err := core.LegacyTableExists(db, core.TableName(config, name))
		if err != nil {
			return "", err
		}
		if ok {
			businessExists = true
			break
		}
	}
	if markerPending || (ledgerExists && !businessExists && !markerFound) {
		return InstallInterrupted, nil
	}
	if ledgerExists && !businessExists && markerFound {
		return "", fmt.Errorf("completed InstallData marker has no business schema")
	}
	if !ledgerExists && !businessExists {
		return InstallFresh, nil
	}
	return InstallStrictUpgrade, nil
}

func IsFreshDatabase(db *gorm.DB, config *conf.Configuration) (bool, error) {
	state, err := DecideInstallRecovery(db, config)
	return state != InstallStrictUpgrade, err
}

func SeedCurrentData(db *gorm.DB, config *conf.Configuration) (bool, error) {
	if err := core.ValidatePrefix(config); err != nil {
		return false, err
	}
	checks := []struct{ table, column, value string }{
		{"admin_rule", "name", "dashboard"}, {"admin_rule", "name", "auth/rule"}, {"admin_rule", "name", "dashboard/index"},
		{"admin_group", "id", "1"}, {"admin", "id", "1"}, {"config", "id", "1"},
	}
	for _, check := range checks {
		t := core.TableName(config, check.table)
		if !core.TableExists(db, t) {
			return false, nil
		}
		var count int64
		if err := db.Table(t).Where(core.QuoteIdentifier(check.column)+" = ?", check.value).Count(&count).Error; err != nil {
			return false, err
		}
		if count != 1 {
			return false, nil
		}
	}
	return true, nil
}
