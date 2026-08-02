package core

import (
	"fmt"

	"go-build-admin/internal/conf"
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
		var record TrackedMigrationRecord
		selectRecord := func() *gorm.DB {
			return db.Table(TableName(config, tableName)).Select("version,migration_name,start_time,end_time,breakpoint")
		}
		q := selectRecord().Where("version = ?", m.Version).First(&record)
		exists := q.Error == nil
		if q.Error != nil && q.Error != gorm.ErrRecordNotFound {
			return count, q.Error
		}
		if exists && record.MigrationName != m.MigrationName {
			return count, fmt.Errorf("%s version %d collision", trackName, m.Version)
		}
		if !exists {
			q = selectRecord().Where("migration_name = ?", m.MigrationName).First(&record)
			if q.Error == nil && record.Version != m.Version {
				return count, fmt.Errorf("%s migration %s collision", trackName, m.MigrationName)
			}
			if q.Error != nil && q.Error != gorm.ErrRecordNotFound {
				return count, q.Error
			}
			if q.Error == gorm.ErrRecordNotFound {
				if err := InsertPendingTrackedMigration(db, config, tableName, m); err != nil {
					return count, err
				}
			}
		} else if record.EndTime != nil {
			if m.VerifySchema != nil {
				if err := m.VerifySchema(db, config); err != nil {
					return count, err
				}
			}
			if m.VerifyUpgradeData != nil {
				if err := m.VerifyUpgradeData(db, config); err != nil {
					return count, err
				}
			}
			continue
		}
		if err := m.Up(db, config); err != nil {
			return count, err
		}
		if m.VerifyBaseline != nil {
			if err := m.VerifyBaseline(db, config); err != nil {
				return count, err
			}
		}
		if m.VerifySchema != nil {
			if err := m.VerifySchema(db, config); err != nil {
				return count, err
			}
		}
		if m.VerifyUpgradeData != nil {
			if err := m.VerifyUpgradeData(db, config); err != nil {
				return count, err
			}
		}
		if err := CompleteTrackedMigration(db, config, tableName, m, trackName); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
