package core

import (
	"fmt"
	"go-build-admin/conf"
	"gorm.io/gorm"
)

type TrackedMigration struct {
	Sequence uint64
	ID       string
	Revision uint64
	Up       MigrationFn
	Down     MigrationFn
	// VerifyBaseline runs only while applying a migration, after Up succeeds.
	// A failed baseline check is retried with the migration; completed records
	// do not run it again. VerifySchema and VerifyUpgradeData are standing runtime
	// invariants and run on every migrate, so their predicates must be business-compatible.
	VerifyBaseline    MigrationFn
	VerifySchema      MigrationFn
	VerifyUpgradeData MigrationFn
}

type TrackedRunnerOptions struct {
	TrackName    string
	AdoptedFrom  func(TrackedMigration) *string
	IncludeBatch bool
	Batch        uint64
}

func RunTrackedMigrations(db *gorm.DB, config *conf.Configuration, tableName string, migrations []TrackedMigration, options TrackedRunnerOptions) (int, error) {
	if err := ValidatePrefix(config); err != nil {
		return 0, err
	}
	trackName := options.TrackName
	if trackName == "" {
		trackName = "tracked"
	}
	batch := uint64(0)
	if options.IncludeBatch {
		batch = options.Batch
		if batch == 0 {
			var err error
			batch, err = NextTrackedBatch(db, config, tableName)
			if err != nil {
				return 0, err
			}
		}
	}
	count := 0
	for _, m := range migrations {
		var record TrackedMigrationRecord
		selectRecord := func() *gorm.DB {
			query := db.Table(TableName(config, tableName)).Select("sequence,migration_id,revision,start_time,end_time")
			if options.IncludeBatch {
				query = query.Select("sequence,batch,migration_id,revision,start_time,end_time")
			}
			return query
		}
		q := selectRecord().Where("sequence = ?", m.Sequence).First(&record)
		exists := q.Error == nil
		if q.Error != nil && q.Error != gorm.ErrRecordNotFound {
			return count, q.Error
		}
		if exists && (record.MigrationID != m.ID || record.Revision != m.Revision) {
			return count, fmt.Errorf("%s sequence %d collision", trackName, m.Sequence)
		}
		if !exists {
			q = selectRecord().Where("migration_id = ?", m.ID).First(&record)
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
				var err error
				if options.IncludeBatch {
					err = InsertPendingTrackedMigrationWithBatch(db, config, tableName, m, batch, adoptedFrom)
				} else {
					err = InsertPendingTrackedMigration(db, config, tableName, m, adoptedFrom)
				}
				if err != nil {
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
