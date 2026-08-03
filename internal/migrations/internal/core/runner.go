package core

import (
	"database/sql"
	"fmt"
	"time"

	"buildadmin-go/internal/pkg/advisorylock"
	"buildadmin-go/internal/conf"
	"gorm.io/gorm"
)

func WithMigrationLock(db *gorm.DB, name string, timeout time.Duration, fn func(*gorm.DB) error) (err error) {
	return advisorylock.With(db, name, timeout, fn)
}

func ValidateMigrationLockRelease(released sql.NullInt64) error {
	if err := advisorylock.ValidateRelease(released); err != nil {
		return fmt.Errorf("migration lock was not released")
	}
	return nil
}

func RunFrameworkMigrations(db *gorm.DB, config *conf.Configuration, official []OfficialMigration, framework []FrameworkMigration) (int, error) {
	if err := ValidateFrameworkMigrations(framework, official); err != nil {
		return 0, err
	}
	if err := ValidatePrefix(config); err != nil {
		return 0, err
	}
	completedOfficial := func(key OfficialKey) error {
		var r MigrationRecord
		q := db.Table(TableName(config, "migrations")).Where("version = ?", key.Version).First(&r)
		if q.Error != nil {
			return q.Error
		}
		if r.MigrationName != key.Name || r.EndTime == nil {
			return fmt.Errorf("required official migration %d/%s is incomplete or collides", key.Version, key.Name)
		}
		return nil
	}
	tracked := make([]TrackedMigration, 0, len(framework))
	for _, m := range framework {
		for _, key := range m.RequiresOfficial {
			if err := completedOfficial(key); err != nil {
				return 0, err
			}
		}
		tracked = append(tracked, TrackedMigration{Version: m.Version, MigrationName: m.MigrationName, Up: m.Up, VerifyBaseline: m.VerifyBaseline, VerifySchema: m.VerifySchema, VerifyUpgradeData: m.VerifyUpgradeData})
	}
	return RunTrackedMigrations(db, config, "migrations_framework", tracked, TrackedRunnerOptions{TrackName: "framework"})
}

// Official ledger operations remain here with the shared runner: moving them
// into official would make core depend on the official package through the
// framework dependency gate and introduce an import cycle.
func BootstrapOfficialLedger(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	return db.Exec("CREATE TABLE IF NOT EXISTS " + QuoteIdentifier(TableName(config, "migrations")) + " (" +
		"`version` BIGINT UNSIGNED NOT NULL, `migration_name` VARCHAR(100) NULL DEFAULT NULL, " +
		"`start_time` TIMESTAMP NULL DEFAULT NULL, `end_time` TIMESTAMP NULL DEFAULT NULL, `breakpoint` TINYINT(1) NOT NULL DEFAULT 0, " +
		"PRIMARY KEY (`version`)) ENGINE=InnoDB").Error
}

func ValidateOfficialLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	// Advisory-lock callers normally provide a fresh handle; isolate direct callers from stale statement state too.
	db = db.Session(&gorm.Session{NewDB: true})
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	var engine string
	if err := db.Raw("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?", TableName(config, "migrations")).Scan(&engine).Error; err != nil {
		return err
	}
	if engine != "InnoDB" {
		return fmt.Errorf("official migrations ledger engine=%q", engine)
	}
	columns, err := queryLedgerColumns(db, TableName(config, "migrations"))
	if err != nil {
		return err
	}
	want := []ledgerColumnSpec{
		{Name: "version", Type: "bigint", Nullable: "NO", AcceptUnsigned: true},
		{Name: "migration_name", Type: "varchar(100)", Nullable: "YES"},
		{Name: "start_time", Type: "timestamp", Nullable: "YES"},
		{Name: "end_time", Type: "timestamp", Nullable: "YES"},
		{Name: "breakpoint", Type: "tinyint(1)", Nullable: "NO"},
	}
	if mismatch := compareLedgerColumns(columns, want); mismatch != nil {
		if mismatch.columnName == "" {
			return fmt.Errorf("official migrations ledger column count=%d", mismatch.actualCount)
		}
		return fmt.Errorf("official migrations ledger schema mismatch at %s", mismatch.columnName)
	}
	return nil
}

func RunOfficialMigrations(db *gorm.DB, config *conf.Configuration, list []OfficialMigration) (int, error) {
	// Advisory-lock callers normally provide a fresh handle; isolate direct callers from stale statement state too.
	db = db.Session(&gorm.Session{NewDB: true})
	if err := ValidateOfficialMigrations(list); err != nil {
		return 0, err
	}
	if err := ValidateOfficialLedgerSchema(db, config); err != nil {
		return 0, err
	}
	count := 0
	for _, migration := range list {
		var record MigrationRecord
		result := db.Table(TableName(config, "migrations")).Where("version = ?", migration.Key.Version).First(&record)
		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			return count, fmt.Errorf("query official %s: %w", migration.Key.Name, result.Error)
		}
		exists := result.Error == nil
		if exists && record.MigrationName != migration.Key.Name {
			return count, fmt.Errorf("official version %d name collision (db=%s, code=%s)", migration.Key.Version, record.MigrationName, migration.Key.Name)
		}
		if exists && record.EndTime != nil {
			continue
		}
		start := time.Now()
		if !exists {
			result = db.Exec("INSERT INTO "+QuoteIdentifier(TableName(config, "migrations"))+" (version, migration_name, start_time, end_time, breakpoint) VALUES (?, ?, ?, NULL, ?)", migration.Key.Version, migration.Key.Name, start, false)
			if result.Error != nil {
				return count, fmt.Errorf("record official %s pending: %w", migration.Key.Name, result.Error)
			}
		}
		if err := migration.Up(db, config); err != nil {
			return count, fmt.Errorf("official migration %s failed: %w", migration.Key.Name, err)
		}
		end := time.Now()
		result = db.Table(TableName(config, "migrations")).Where("version = ? AND migration_name = ? AND end_time IS NULL", migration.Key.Version, migration.Key.Name).Update("end_time", end)
		if result.Error != nil {
			return count, fmt.Errorf("record official %s: %w", migration.Key.Name, result.Error)
		}
		if result.RowsAffected != 1 {
			return count, fmt.Errorf("official %s completion affected %d rows", migration.Key.Name, result.RowsAffected)
		}
		count++
	}
	return count, nil
}
