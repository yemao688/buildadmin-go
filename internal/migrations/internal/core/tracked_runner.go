package core

import (
	"errors"
	"fmt"

	"buildadmin-go/internal/conf"
	"gorm.io/gorm"
)

type TrackedMigration struct {
	Version           uint64
	MigrationName     string
	Up                MigrationFn
	Down              MigrationFn
	VerifyBaseline    MigrationFn
	VerifySchema      MigrationFn
	VerifyUpgradeData MigrationFn
}

type TrackedRunnerOptions struct {
	TrackName string
}

func RunTrackedMigrations(db *gorm.DB, config *conf.Configuration, tableName string, migrations []TrackedMigration, options TrackedRunnerOptions) (int, error) {
	if err := ValidatePrefix(config); err != nil {
		return 0, err
	}
	trackName := options.TrackName
	if trackName == "" {
		trackName = "tracked"
	}
	count := 0
	for _, m := range migrations {
		record, exists, err := trackedMigrationRecord(db, config, tableName, trackName, m)
		if err != nil {
			return count, err
		}
		if exists && record.EndTime != nil {
			if err := verifyTrackedMigration(db, config, m, false); err != nil {
				return count, err
			}
			continue
		}
		if err := applyTrackedMigration(db, config, tableName, trackName, m); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// trackedMigrationRecord 定位版本对应的账本行，校验版本/名称冲突，并在完全
// 缺失时写入 pending 记录。exists 为 false 表示该版本尚未完成（含刚写入
// pending 的情况），调用方应执行 Up。
func trackedMigrationRecord(db *gorm.DB, config *conf.Configuration, tableName, trackName string, m TrackedMigration) (TrackedMigrationRecord, bool, error) {
	var record TrackedMigrationRecord
	selectRecord := func() *gorm.DB {
		return db.Table(TableName(config, tableName)).Select("version,migration_name,start_time,end_time,breakpoint")
	}
	q := selectRecord().Where("version = ?", m.Version).First(&record)
	exists := q.Error == nil
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return record, false, q.Error
	}
	if exists && record.MigrationName != m.MigrationName {
		return record, false, fmt.Errorf("%s version %d collision", trackName, m.Version)
	}
	if exists {
		return record, true, nil
	}
	q = selectRecord().Where("migration_name = ?", m.MigrationName).First(&record)
	if q.Error == nil && record.Version != m.Version {
		return record, false, fmt.Errorf("%s migration %s collision", trackName, m.MigrationName)
	}
	if q.Error != nil && !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return record, false, q.Error
	}
	if errors.Is(q.Error, gorm.ErrRecordNotFound) {
		if err := InsertPendingTrackedMigration(db, config, tableName, m); err != nil {
			return record, false, err
		}
	}
	return record, false, nil
}

// verifyTrackedMigration 运行常驻校验；includeBaseline 决定是否执行只在
// Up 应用后运行一次的基线判据。
func verifyTrackedMigration(db *gorm.DB, config *conf.Configuration, m TrackedMigration, includeBaseline bool) error {
	if includeBaseline && m.VerifyBaseline != nil {
		if err := m.VerifyBaseline(db, config); err != nil {
			return err
		}
	}
	if m.VerifySchema != nil {
		if err := m.VerifySchema(db, config); err != nil {
			return err
		}
	}
	if m.VerifyUpgradeData != nil {
		if err := m.VerifyUpgradeData(db, config); err != nil {
			return err
		}
	}
	return nil
}

func applyTrackedMigration(db *gorm.DB, config *conf.Configuration, tableName, trackName string, m TrackedMigration) error {
	if err := m.Up(db, config); err != nil {
		return err
	}
	if err := verifyTrackedMigration(db, config, m, true); err != nil {
		return err
	}
	return CompleteTrackedMigration(db, config, tableName, m, trackName)
}
