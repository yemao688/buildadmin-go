package core

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

func WithMigrationLock(db *gorm.DB, name string, timeout time.Duration, fn func(*gorm.DB) error) (err error) {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	var got int
	if err = conn.QueryRowContext(context.Background(), "SELECT GET_LOCK(?, ?)", name, int(timeout/time.Second)).Scan(&got); err != nil {
		return err
	}
	if got != 1 {
		return fmt.Errorf("could not acquire migration lock %q", name)
	}
	defer func() {
		releaseErr := releaseMigrationLock(conn, name)
		if err == nil && releaseErr != nil {
			err = releaseErr
		}
	}()
	callbackDB := db.Session(&gorm.Session{NewDB: true, Context: context.Background()})
	callbackDB.Statement.ConnPool = conn
	return fn(callbackDB)
}

func releaseMigrationLock(conn *sql.Conn, name string) error {
	var released sql.NullInt64
	if err := conn.QueryRowContext(context.Background(), "SELECT RELEASE_LOCK(?)", name).Scan(&released); err != nil {
		return err
	}
	return ValidateMigrationLockRelease(released)
}

func ValidateMigrationLockRelease(released sql.NullInt64) error {
	if !released.Valid || released.Int64 != 1 {
		return fmt.Errorf("migration lock was not released")
	}
	return nil
}

func RunLocalMigrations(db *gorm.DB, config *conf.Configuration, official []OfficialMigration, local []LocalMigration) (int, error) {
	if err := ValidateLocalMigrations(local, official); err != nil {
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
	tracked := make([]TrackedMigration, 0, len(local))
	for _, m := range local {
		for _, key := range m.RequiresOfficial {
			if err := completedOfficial(key); err != nil {
				return 0, err
			}
		}
		tracked = append(tracked, TrackedMigration{Sequence: m.Sequence, ID: m.ID, Revision: m.Revision, Up: m.Up, VerifySchema: m.VerifySchema, VerifyData: m.VerifyUpgradeData})
	}
	return RunTrackedMigrations(db, config, "go_migrations", tracked, TrackedRunnerOptions{TrackName: "local"})
}

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
	var columns []struct{ Name, Type, Nullable string }
	if err := db.Raw("SELECT COLUMN_NAME AS name, COLUMN_TYPE AS type, IS_NULLABLE AS nullable FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? ORDER BY ORDINAL_POSITION", TableName(config, "migrations")).Scan(&columns).Error; err != nil {
		return err
	}
	want := []struct{ name, typ, nullable string }{{"version", "bigint", "NO"}, {"migration_name", "varchar(100)", "YES"}, {"start_time", "timestamp", "YES"}, {"end_time", "timestamp", "YES"}, {"breakpoint", "tinyint(1)", "NO"}}
	if len(columns) != len(want) {
		return fmt.Errorf("official migrations ledger column count=%d", len(columns))
	}
	for i, column := range columns {
		if column.Name != want[i].name || (i == 0 && column.Type != "bigint" && column.Type != "bigint unsigned") || (i != 0 && column.Type != want[i].typ) || column.Nullable != want[i].nullable {
			return fmt.Errorf("official migrations ledger schema mismatch at %s", column.Name)
		}
	}
	return nil
}

func RunOfficialMigrations(db *gorm.DB, config *conf.Configuration, list []OfficialMigration) (int, error) {
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
		if err := migration.Up(db, config); err != nil {
			return count, fmt.Errorf("official migration %s failed: %w", migration.Key.Name, err)
		}
		end := time.Now()
		if exists {
			result = db.Table(TableName(config, "migrations")).Where("version = ? AND migration_name = ? AND end_time IS NULL", migration.Key.Version, migration.Key.Name).Updates(map[string]any{"start_time": start, "end_time": end})
		} else {
			result = db.Exec("INSERT INTO "+QuoteIdentifier(TableName(config, "migrations"))+" (version, migration_name, start_time, end_time, breakpoint) VALUES (?, ?, ?, ?, ?)", migration.Key.Version, migration.Key.Name, start, end, false)
		}
		if result.Error != nil {
			return count, fmt.Errorf("record official %s: %w", migration.Key.Name, result.Error)
		}
		if exists && result.RowsAffected != 1 {
			return count, fmt.Errorf("official %s completion affected %d rows", migration.Key.Name, result.RowsAffected)
		}
		count++
	}
	return count, nil
}
