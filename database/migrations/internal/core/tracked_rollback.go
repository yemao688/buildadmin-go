package core

import (
	"fmt"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

type RollbackOptions struct {
	Steps         uint64
	TargetVersion *uint64
	TrackName     string
}

type RollbackEntry struct {
	Version       uint64
	MigrationName string
	// These aliases preserve the existing CLI output contract. They are not
	// used for ledger queries or rollback ordering.
	Sequence      uint64
	ID            string
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
	if options.Steps > 0 && options.TargetVersion != nil {
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
	query := db.Table(TableName(config, tableName)).Select("version,migration_name,start_time,end_time,breakpoint").Where("end_time IS NOT NULL")
	if options.TargetVersion != nil {
		query = query.Where("version > ?", *options.TargetVersion)
	}
	if err := query.Order("version DESC").Find(&rows).Error; err != nil {
		return RollbackReport{}, err
	}
	if options.TargetVersion == nil && options.Steps == 0 && len(rows) > 1 {
		rows = rows[:1]
	}
	if options.Steps > 0 && uint64(len(rows)) > options.Steps {
		rows = rows[:options.Steps]
	}
	if len(rows) == 0 {
		return RollbackReport{}, nil
	}

	byVersion := make(map[uint64]TrackedMigration, len(migrations))
	for _, migration := range migrations {
		byVersion[migration.Version] = migration
	}
	report := RollbackReport{Entries: make([]RollbackEntry, len(rows))}
	for i, row := range rows {
		migration, ok := byVersion[row.Version]
		if !ok || migration.MigrationName != row.MigrationName {
			return report, fmt.Errorf("business migration ledger entry version %d/%s is missing from or collides with the registered migrations", row.Version, row.MigrationName)
		}
		report.Entries[i] = RollbackEntry{
			Version:       row.Version,
			MigrationName: row.MigrationName,
			Sequence:      row.Version,
			ID:            row.MigrationName,
		}
		if migration.Down == nil {
			return report, fmt.Errorf("business migration %s (version %d) has no Down function", row.MigrationName, row.Version)
		}
	}

	for i := range report.Entries {
		entry := &report.Entries[i]
		migration := byVersion[entry.Version]
		if err := migration.Down(db, config); err != nil {
			entry.Error = fmt.Sprintf("Down failed: %v", err)
			return report, fmt.Errorf("business migration %s rollback failed: %w (rolled back %d, not rolled back %d)", entry.MigrationName, err, report.RolledBack(), report.NotRolledBack())
		}
		entry.DownExecuted = true
		result := db.Exec("DELETE FROM "+table+" WHERE version = ? AND migration_name = ? AND end_time IS NOT NULL", entry.Version, entry.MigrationName)
		if result.Error != nil {
			entry.Error = fmt.Sprintf("ledger delete failed: %v", result.Error)
			return report, fmt.Errorf("business migration %s Down succeeded but ledger removal failed: %w (rolled back %d, not rolled back %d)", entry.MigrationName, result.Error, report.RolledBack(), report.NotRolledBack())
		}
		if result.RowsAffected != 1 {
			entry.Error = fmt.Sprintf("ledger delete affected %d rows", result.RowsAffected)
			return report, fmt.Errorf("business migration %s ledger removal affected %d rows after Down (rolled back %d, not rolled back %d)", entry.MigrationName, result.RowsAffected, report.RolledBack(), report.NotRolledBack())
		}
		entry.LedgerRemoved = true
	}
	return report, nil
}
