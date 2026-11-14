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
	markerFound, markerPending, err := installSeedMarkerState(db, config, ledgerExists)
	if err != nil {
		return "", err
	}
	businessExists, err := installBusinessSchemaExists(db, config)
	if err != nil {
		return "", err
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

// installSeedMarkerState 读取官方账本里的 InstallData 标记行：found 表示标记行
// 存在且名字匹配，pending 表示该标记尚未完成。
func installSeedMarkerState(db *gorm.DB, config *conf.Configuration, ledgerExists bool) (found, pending bool, err error) {
	if !ledgerExists {
		return false, false, nil
	}
	var marker core.MigrationRecord
	result := db.Table(core.TableName(config, "migrations")).Where("version = ?", InstallDataVersion).First(&marker)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, false, result.Error
	}
	if result.Error == nil {
		if marker.MigrationName != InstallDataName {
			return false, false, fmt.Errorf("migration version %d name collision", InstallDataVersion)
		}
		return true, marker.EndTime == nil, nil
	}
	return false, false, nil
}

func installBusinessSchemaExists(db *gorm.DB, config *conf.Configuration) (bool, error) {
	for _, name := range core.CoreLogicalNames() {
		if name == "migrations" {
			// The official ledger tracks installation state; it is not business schema.
			continue
		}
		ok, err := core.LegacyTableExists(db, core.TableName(config, name))
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
