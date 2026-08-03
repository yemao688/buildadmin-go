package core

import (
	"fmt"
	"time"

	"buildadmin-go/internal/conf"
	"gorm.io/gorm"
)

type TrackedMigrationRecord struct {
	Version       uint64     `gorm:"column:version"`
	MigrationName string     `gorm:"column:migration_name"`
	StartTime     time.Time  `gorm:"column:start_time"`
	EndTime       *time.Time `gorm:"column:end_time"`
	Breakpoint    bool       `gorm:"column:breakpoint"`
}

func BootstrapTrackedLedger(db *gorm.DB, config *conf.Configuration, logicalName string) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	return db.Exec("CREATE TABLE IF NOT EXISTS " + QuoteIdentifier(TableName(config, logicalName)) + " (" +
		"`version` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, " +
		"`migration_name` VARCHAR(191) NOT NULL, " +
		"`start_time` TIMESTAMP(6) NOT NULL, " +
		"`end_time` TIMESTAMP(6) NULL DEFAULT NULL, " +
		"`breakpoint` TINYINT(1) NOT NULL DEFAULT 0, " +
		"PRIMARY KEY (`version`), UNIQUE KEY `uq_" + logicalName + "_migration_name` (`migration_name`)) ENGINE=InnoDB").Error
}

func ValidateTrackedLedgerSchema(db *gorm.DB, config *conf.Configuration, logicalName string) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	table := TableName(config, logicalName)
	var engine string
	if err := db.Raw("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table).Scan(&engine).Error; err != nil {
		return err
	}
	if engine != "InnoDB" {
		return fmt.Errorf("%s schema mismatch: engine=%q", logicalName, engine)
	}
	rows, err := queryLedgerColumns(db, table)
	if err != nil {
		return err
	}
	defaultZero := "0"
	want := []ledgerColumnSpec{
		{Name: "version", Type: "bigint unsigned", Nullable: "NO", CheckPrecision: true, RequireNoDefault: true, RequireAutoIncrement: true},
		{Name: "migration_name", Type: "varchar(191)", Nullable: "NO", CheckPrecision: true, RequireNoDefault: true},
		{Name: "start_time", Type: "timestamp(6)", Nullable: "NO", Precision: 6, CheckPrecision: true, RequireNoDefault: true},
		{Name: "end_time", Type: "timestamp(6)", Nullable: "YES", Precision: 6, CheckPrecision: true, RequireNoDefault: true},
		{Name: "breakpoint", Type: "tinyint(1)", Nullable: "NO", CheckPrecision: true, ExpectedDefault: &defaultZero},
	}
	if mismatch := compareLedgerColumns(rows, want); mismatch != nil {
		if mismatch.columnName == "" {
			return fmt.Errorf("%s schema mismatch: got %d columns", logicalName, mismatch.actualCount)
		}
		return fmt.Errorf("%s schema mismatch at %s", logicalName, mismatch.columnName)
	}
	type indexColumn struct {
		Name, Column string
		NonUnique    int
	}
	var indexes []indexColumn
	if err := db.Raw("SELECT INDEX_NAME AS name, COLUMN_NAME AS `column`, NON_UNIQUE AS non_unique FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY INDEX_NAME, SEQ_IN_INDEX", table).Scan(&indexes).Error; err != nil {
		return err
	}
	uniqueName := "uq_" + logicalName + "_migration_name"
	if len(indexes) != 2 || indexes[0].Name != "PRIMARY" || indexes[0].Column != "version" || indexes[0].NonUnique != 0 || indexes[1].Name != uniqueName || indexes[1].Column != "migration_name" || indexes[1].NonUnique != 0 {
		return fmt.Errorf("%s indexes mismatch", logicalName)
	}
	return nil
}

// Framework ledger operations are tracked ledgers with a fixed logical name.
func BootstrapFrameworkLedger(db *gorm.DB, config *conf.Configuration) error {
	return BootstrapTrackedLedger(db, config, "migrations_framework")
}

func ValidateFrameworkLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	return ValidateTrackedLedgerSchema(db, config, "migrations_framework")
}

func InsertPendingFrameworkMigration(db *gorm.DB, config *conf.Configuration, m FrameworkMigration) error {
	return InsertPendingTrackedMigration(db, config, "migrations_framework", TrackedMigration{Version: m.Version, MigrationName: m.MigrationName, Up: m.Up})
}

func CompleteFrameworkMigration(db *gorm.DB, config *conf.Configuration, m FrameworkMigration) error {
	return CompleteTrackedMigration(db, config, "migrations_framework", TrackedMigration{Version: m.Version, MigrationName: m.MigrationName}, "framework")
}

func InsertPendingTrackedMigration(db *gorm.DB, config *conf.Configuration, logicalName string, m TrackedMigration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	return db.Exec("INSERT INTO "+QuoteIdentifier(TableName(config, logicalName))+" (version,migration_name,start_time) VALUES (?,?,?)", m.Version, m.MigrationName, time.Now()).Error
}

func CompleteTrackedMigration(db *gorm.DB, config *conf.Configuration, logicalName string, m TrackedMigration, trackName string) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	result := db.Table(TableName(config, logicalName)).Where("migration_name = ? AND end_time IS NULL", m.MigrationName).Update("end_time", time.Now())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%s migration %s completion identity/pending mismatch", trackName, m.MigrationName)
	}
	return nil
}
