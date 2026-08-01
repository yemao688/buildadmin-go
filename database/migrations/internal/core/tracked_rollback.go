package core

import (
	"fmt"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

type RollbackOptions struct {
	Steps          uint64
	TargetSequence *uint64
	TrackName      string
}

type RollbackEntry struct {
	Sequence      uint64
	ID            string
	Revision      uint64
	Batch         uint64
	DownExecuted  bool
	LedgerRemoved bool
	Error         string
}

type RollbackReport struct {
	Entries []RollbackEntry
}

func (r RollbackReport) RolledBack() int {
	count := 0
	for _, entry := range r.Entries {
		if entry.DownExecuted && entry.LedgerRemoved {
			count++
		}
	}
	return count
}

func (r RollbackReport) NotRolledBack() int { return len(r.Entries) - r.RolledBack() }

func RollbackTrackedMigrations(db *gorm.DB, config *conf.Configuration, tableName string, migrations []TrackedMigration, options RollbackOptions) (RollbackReport, error) {
	if options.TrackName != "business" {
		trackName := options.TrackName
		if trackName == "" {
			trackName = "unknown"
		}
		return RollbackReport{}, fmt.Errorf("%s migration rollback is unsupported; official and framework migrations are forward-only", trackName)
	}
	if err := ValidatePrefix(config); err != nil {
		return RollbackReport{}, err
	}
	if options.Steps > 0 && options.TargetSequence != nil {
		return RollbackReport{}, fmt.Errorf("rollback steps cannot be combined with a breakpoint target")
	}
	table := QuoteIdentifier(TableName(config, tableName))
	var pending int64
	if err := db.Table(TableName(config, tableName)).Where("end_time IS NULL").Count(&pending).Error; err != nil {
		return RollbackReport{}, err
	}
	if pending != 0 {
		return RollbackReport{}, fmt.Errorf("business migration ledger contains %d incomplete migration(s); complete or repair them before rollback", pending)
	}

	var rows []TrackedMigrationRecord
	query := db.Table(TableName(config, tableName)).Select("sequence,batch,migration_id,revision,start_time,end_time").Where("end_time IS NOT NULL")
	if options.TargetSequence != nil {
		query = query.Where("sequence > ?", *options.TargetSequence)
	}
	if err := query.Order("batch DESC, sequence DESC").Find(&rows).Error; err != nil {
		return RollbackReport{}, err
	}
	if options.TargetSequence == nil && options.Steps == 0 && len(rows) > 0 {
		latestBatch := rows[0].Batch
		filtered := rows[:0]
		for _, row := range rows {
			if row.Batch == latestBatch {
				filtered = append(filtered, row)
			}
		}
		rows = filtered
	}
	if options.Steps > 0 && uint64(len(rows)) > options.Steps {
		rows = rows[:options.Steps]
	}
	if len(rows) == 0 {
		return RollbackReport{}, nil
	}

	bySequence := make(map[uint64]TrackedMigration, len(migrations))
	for _, migration := range migrations {
		bySequence[migration.Sequence] = migration
	}
	report := RollbackReport{Entries: make([]RollbackEntry, len(rows))}
	for i, row := range rows {
		migration, ok := bySequence[row.Sequence]
		if !ok || migration.ID != row.MigrationID || migration.Revision != row.Revision {
			return report, fmt.Errorf("business migration ledger entry sequence %d/%s is missing from or collides with the registered migrations", row.Sequence, row.MigrationID)
		}
		report.Entries[i] = RollbackEntry{Sequence: row.Sequence, ID: row.MigrationID, Revision: row.Revision, Batch: row.Batch}
		if migration.Down == nil {
			return report, fmt.Errorf("business migration %s (sequence %d) has no Down function", row.MigrationID, row.Sequence)
		}
	}

	for i := range report.Entries {
		entry := &report.Entries[i]
		migration := bySequence[entry.Sequence]
		if err := migration.Down(db, config); err != nil {
			entry.Error = fmt.Sprintf("Down failed: %v", err)
			return report, fmt.Errorf("business migration %s rollback failed: %w (rolled back %d, not rolled back %d)", entry.ID, err, report.RolledBack(), report.NotRolledBack())
		}
		entry.DownExecuted = true
		result := db.Exec("DELETE FROM "+table+" WHERE sequence = ? AND migration_id = ? AND revision = ? AND end_time IS NOT NULL", entry.Sequence, entry.ID, entry.Revision)
		if result.Error != nil {
			entry.Error = fmt.Sprintf("ledger delete failed: %v", result.Error)
			return report, fmt.Errorf("business migration %s Down succeeded but ledger removal failed: %w (rolled back %d, not rolled back %d)", entry.ID, result.Error, report.RolledBack(), report.NotRolledBack())
		}
		if result.RowsAffected != 1 {
			entry.Error = fmt.Sprintf("ledger delete affected %d rows", result.RowsAffected)
			return report, fmt.Errorf("business migration %s ledger removal affected %d rows after Down (rolled back %d, not rolled back %d)", entry.ID, result.RowsAffected, report.RolledBack(), report.NotRolledBack())
		}
		entry.LedgerRemoved = true
	}
	return report, nil
}
