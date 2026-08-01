package core

import (
	"fmt"
	"time"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

type TrackedMigrationRecord struct {
	Sequence    uint64     `gorm:"column:sequence"`
	Batch       uint64     `gorm:"column:batch"`
	MigrationID string     `gorm:"column:migration_id"`
	Revision    uint64     `gorm:"column:revision"`
	StartTime   time.Time  `gorm:"column:start_time"`
	EndTime     *time.Time `gorm:"column:end_time"`
}

type TrackedLedgerOptions struct {
	IncludeBatch bool
}

func BootstrapTrackedLedger(db *gorm.DB, config *conf.Configuration, logicalName string, options TrackedLedgerOptions) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	batch := ""
	if options.IncludeBatch {
		batch = "`batch` BIGINT UNSIGNED NOT NULL, "
	}
	if err := db.Exec("CREATE TABLE IF NOT EXISTS " + QuoteIdentifier(TableName(config, logicalName)) + " (" +
		"`sequence` BIGINT UNSIGNED NOT NULL, " + batch + "`migration_id` VARCHAR(191) NOT NULL, `revision` BIGINT UNSIGNED NOT NULL, " +
		"`start_time` TIMESTAMP(6) NOT NULL, `end_time` TIMESTAMP(6) NULL DEFAULT NULL" +
		", PRIMARY KEY (`sequence`), UNIQUE KEY `uq_" + logicalName + "_id` (`migration_id`)) ENGINE=InnoDB").Error; err != nil {
		return err
	}
	if !options.IncludeBatch {
		return nil
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = 'batch'", TableName(config, logicalName)).Scan(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	table := QuoteIdentifier(TableName(config, logicalName))
	if err := db.Exec("ALTER TABLE " + table + " ADD COLUMN `batch` BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER `sequence`").Error; err != nil {
		return err
	}
	if err := db.Exec("UPDATE " + table + " SET `batch` = `sequence` WHERE `batch` = 0").Error; err != nil {
		return err
	}
	return db.Exec("ALTER TABLE " + table + " MODIFY COLUMN `batch` BIGINT UNSIGNED NOT NULL").Error
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
	}
	if options.IncludeBatch {
		want = append(want, ledgerColumnSpec{Name: "batch", Type: "bigint unsigned", Nullable: "NO", CheckPrecision: true, RequireNoDefault: true})
	}
	want = append(want,
		ledgerColumnSpec{Name: "migration_id", Type: "varchar(191)", Nullable: "NO", CheckPrecision: true, RequireNoDefault: true},
		ledgerColumnSpec{Name: "revision", Type: "bigint unsigned", Nullable: "NO", CheckPrecision: true, RequireNoDefault: true},
		ledgerColumnSpec{Name: "start_time", Type: "timestamp(6)", Nullable: "NO", Precision: 6, CheckPrecision: true, RequireNoDefault: true},
		ledgerColumnSpec{Name: "end_time", Type: "timestamp(6)", Nullable: "YES", Precision: 6, CheckPrecision: true, RequireNoDefault: true},
	)
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

func InsertPendingTrackedMigration(db *gorm.DB, config *conf.Configuration, logicalName string, m TrackedMigration) error {
	return insertPendingTrackedMigration(db, config, logicalName, m, 0)
}

func InsertPendingTrackedMigrationWithBatch(db *gorm.DB, config *conf.Configuration, logicalName string, m TrackedMigration, batch uint64) error {
	if batch == 0 {
		return fmt.Errorf("%s migration %s has invalid batch 0", logicalName, m.ID)
	}
	return insertPendingTrackedMigration(db, config, logicalName, m, batch)
}

func insertPendingTrackedMigration(db *gorm.DB, config *conf.Configuration, logicalName string, m TrackedMigration, batch uint64) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	record := TrackedMigrationRecord{Sequence: m.Sequence, MigrationID: m.ID, Revision: m.Revision, StartTime: time.Now()}
	table := QuoteIdentifier(TableName(config, logicalName))
	if batch != 0 {
		record.Batch = batch
		return db.Table(TableName(config, logicalName)).Exec("INSERT INTO "+table+" (sequence,batch,migration_id,revision,start_time) VALUES (?,?,?,?,?)", record.Sequence, record.Batch, record.MigrationID, record.Revision, record.StartTime).Error
	}
	return db.Table(TableName(config, logicalName)).Exec("INSERT INTO "+table+" (sequence,migration_id,revision,start_time) VALUES (?,?,?,?)", record.Sequence, record.MigrationID, record.Revision, record.StartTime).Error
}

func NextTrackedBatch(db *gorm.DB, config *conf.Configuration, logicalName string) (uint64, error) {
	if err := ValidatePrefix(config); err != nil {
		return 0, err
	}
	var maximum uint64
	if err := db.Table(TableName(config, logicalName)).Select("COALESCE(MAX(`batch`), 0)").Scan(&maximum).Error; err != nil {
		return 0, err
	}
	if maximum == 0 {
		return 1, nil
	}
	if maximum == ^uint64(0) {
		return 0, fmt.Errorf("%s migration batch overflow", logicalName)
	}
	return maximum + 1, nil
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
