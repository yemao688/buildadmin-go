package core

import (
	"fmt"
	"go-build-admin/conf"
	"gorm.io/gorm"
)

type TrackedMigration struct {
	Sequence     uint64
	ID           string
	Revision     uint64
	Up           MigrationFn
	VerifySchema MigrationFn
	VerifyData   MigrationFn
}

type TrackedRunnerOptions struct {
	TrackName   string
	AdoptedFrom func(TrackedMigration) *string
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
		q := db.Table(TableName(config, tableName)).Where("sequence = ?", m.Sequence).First(&record)
		exists := q.Error == nil
		if q.Error != nil && q.Error != gorm.ErrRecordNotFound {
			return count, q.Error
		}
		if exists && (record.MigrationID != m.ID || record.Revision != m.Revision) {
			return count, fmt.Errorf("%s sequence %d collision", trackName, m.Sequence)
		}
		if !exists {
			q = db.Table(TableName(config, tableName)).Where("migration_id = ?", m.ID).First(&record)
			if q.Error == nil && (record.Sequence != m.Sequence || record.Revision != m.Revision) {
				return count, fmt.Errorf("%s migration %s collision", trackName, m.ID)
			}
			if q.Error != nil && q.Error != gorm.ErrRecordNotFound {
				return count, q.Error
			}
			if q.Error == gorm.ErrRecordNotFound {
				var adoptedFrom *string
				if options.AdoptedFrom != nil {
					adoptedFrom = options.AdoptedFrom(m)
				}
				if err := InsertPendingTrackedMigration(db, config, tableName, m, adoptedFrom); err != nil {
					return count, err
				}
			}
		} else if record.EndTime != nil {
			if m.VerifySchema != nil {
				if err := m.VerifySchema(db, config); err != nil {
					return count, err
				}
			}
			if m.VerifyData != nil {
				if err := m.VerifyData(db, config); err != nil {
					return count, err
				}
			}
			continue
		}
		if err := m.Up(db, config); err != nil {
			return count, err
		}
		if m.VerifySchema != nil {
			if err := m.VerifySchema(db, config); err != nil {
				return count, err
			}
		}
		if m.VerifyData != nil {
			if err := m.VerifyData(db, config); err != nil {
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
