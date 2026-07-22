package official

import (
	"errors"
	"fmt"
	"time"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/internal/core"
	"gorm.io/gorm"
)

func MarkSeedPending(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	db = db.Session(&gorm.Session{NewDB: true})
	table := core.TableName(config, "migrations")
	var record core.MigrationRecord
	result := db.Table(table).Where("version = ?", InstallDataVersion).First(&record)
	if result.Error == nil {
		if record.MigrationName != InstallDataName {
			return fmt.Errorf("migration version %d name collision", InstallDataVersion)
		}
		if record.EndTime == nil {
			return nil
		}
		return nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return result.Error
	}
	now := time.Now()
	return db.Exec("INSERT INTO "+core.QuoteIdentifier(core.TableName(config, "migrations"))+" (version, migration_name, start_time, end_time, breakpoint) VALUES (?, ?, ?, NULL, ?)", InstallDataVersion, InstallDataName, now, false).Error
}

func SeedPending(db *gorm.DB, config *conf.Configuration) (bool, error) {
	if err := core.ValidatePrefix(config); err != nil {
		return false, err
	}
	db = db.Session(&gorm.Session{NewDB: true})
	var record core.MigrationRecord
	result := db.Table(core.TableName(config, "migrations")).Where("version = ?", InstallDataVersion).First(&record)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if result.Error != nil {
		return false, result.Error
	}
	if record.MigrationName != InstallDataName {
		return false, fmt.Errorf("migration version %d name collision", InstallDataVersion)
	}
	return record.EndTime == nil, nil
}

func MarkSeedCompleted(db *gorm.DB, config *conf.Configuration) error {
	if err := core.ValidatePrefix(config); err != nil {
		return err
	}
	db = db.Session(&gorm.Session{NewDB: true})
	now := time.Now()
	result := db.Table(core.TableName(config, "migrations")).Where("version = ? AND migration_name = ?", InstallDataVersion, InstallDataName).Updates(map[string]any{"end_time": now, "start_time": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("pending %s marker not found", InstallDataName)
	}
	return nil
}
