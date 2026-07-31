package core

import (
	"fmt"
	"time"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

type TrackedMigrationRecord struct {
	Sequence    uint64     `gorm:"column:sequence"`
	MigrationID string     `gorm:"column:migration_id"`
	Revision    uint64     `gorm:"column:revision"`
	StartTime   time.Time  `gorm:"column:start_time"`
	EndTime     *time.Time `gorm:"column:end_time"`
}

type TrackedLedgerOptions struct {
	IncludeAdoptedFrom bool
}

func BootstrapTrackedLedger(db *gorm.DB, config *conf.Configuration, logicalName string, options TrackedLedgerOptions) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	adopted := ""
	if options.IncludeAdoptedFrom {
		adopted = ", `adopted_from` VARCHAR(191) NULL DEFAULT NULL"
	}
	return db.Exec("CREATE TABLE IF NOT EXISTS " + QuoteIdentifier(TableName(config, logicalName)) + " (" +
		"`sequence` BIGINT UNSIGNED NOT NULL, `migration_id` VARCHAR(191) NOT NULL, `revision` BIGINT UNSIGNED NOT NULL, " +
		"`start_time` TIMESTAMP(6) NOT NULL, `end_time` TIMESTAMP(6) NULL DEFAULT NULL" + adopted +
		", PRIMARY KEY (`sequence`), UNIQUE KEY `uq_" + logicalName + "_id` (`migration_id`)) ENGINE=InnoDB").Error
}

func ValidateTrackedLedgerSchema(db *gorm.DB, config *conf.Configuration, logicalName string, options TrackedLedgerOptions) error {
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
	want := []ledgerColumnSpec{
		{Name: "sequence", Type: "bigint unsigned", Nullable: "NO", CheckPrecision: true, RequireNoDefault: true},
		{Name: "migration_id", Type: "varchar(191)", Nullable: "NO", CheckPrecision: true, RequireNoDefault: true},
		{Name: "revision", Type: "bigint unsigned", Nullable: "NO", CheckPrecision: true, RequireNoDefault: true},
		{Name: "start_time", Type: "timestamp(6)", Nullable: "NO", Precision: 6, CheckPrecision: true, RequireNoDefault: true},
		{Name: "end_time", Type: "timestamp(6)", Nullable: "YES", Precision: 6, CheckPrecision: true, RequireNoDefault: true},
	}
	if options.IncludeAdoptedFrom {
		want = append(want, ledgerColumnSpec{Name: "adopted_from", Type: "varchar(191)", Nullable: "YES", CheckPrecision: true, RequireNoDefault: true})
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
	uniqueName := "uq_" + logicalName + "_id"
	if len(indexes) != 2 || indexes[0].Name != "PRIMARY" || indexes[0].Column != "sequence" || indexes[0].NonUnique != 0 || indexes[1].Name != uniqueName || indexes[1].Column != "migration_id" || indexes[1].NonUnique != 0 {
		return fmt.Errorf("%s indexes mismatch", logicalName)
	}
	return nil
}

func InsertPendingTrackedMigration(db *gorm.DB, config *conf.Configuration, logicalName string, m TrackedMigration, adoptedFrom *string) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	record := TrackedMigrationRecord{Sequence: m.Sequence, MigrationID: m.ID, Revision: m.Revision, StartTime: time.Now()}
	if adoptedFrom != nil {
		return db.Table(TableName(config, logicalName)).Exec("INSERT INTO "+QuoteIdentifier(TableName(config, logicalName))+" (sequence,migration_id,revision,start_time,adopted_from) VALUES (?,?,?,?,?)", record.Sequence, record.MigrationID, record.Revision, record.StartTime, adoptedFrom).Error
	}
	return db.Table(TableName(config, logicalName)).Create(&record).Error
}

func CompleteTrackedMigration(db *gorm.DB, config *conf.Configuration, logicalName string, m TrackedMigration, trackName string) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	result := db.Table(TableName(config, logicalName)).Where("sequence = ? AND migration_id = ? AND revision = ? AND end_time IS NULL", m.Sequence, m.ID, m.Revision).Update("end_time", time.Now())
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%s migration %s completion identity/revision/pending mismatch", trackName, m.ID)
	}
	return nil
}
