package core

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

func localLedgerTable(config *conf.Configuration) string {
	return QuoteIdentifier(TableName(config, "go_migrations"))
}

func BootstrapLocalLedger(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	return db.Exec("CREATE TABLE IF NOT EXISTS " + localLedgerTable(config) + " (" +
		"`sequence` BIGINT UNSIGNED NOT NULL, `migration_id` VARCHAR(191) NOT NULL, `revision` BIGINT UNSIGNED NOT NULL, " +
		"`start_time` TIMESTAMP(6) NOT NULL, `end_time` TIMESTAMP(6) NULL DEFAULT NULL, `adopted_from` VARCHAR(191) NULL DEFAULT NULL, " +
		"PRIMARY KEY (`sequence`), UNIQUE KEY `uq_go_migrations_id` (`migration_id`)) ENGINE=InnoDB").Error
}

func ValidateLocalLedgerSchema(db *gorm.DB, config *conf.Configuration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	var engine string
	if err := db.Raw("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", TableName(config, "go_migrations")).Scan(&engine).Error; err != nil {
		return err
	}
	if engine != "InnoDB" {
		return fmt.Errorf("go_migrations schema mismatch: engine=%q", engine)
	}
	type column struct {
		Name, Type, IsNullable string
		Precision              sql.NullInt64
		Default                sql.NullString
	}
	var rows []column
	if err := db.Raw("SELECT COLUMN_NAME AS name, COLUMN_TYPE AS type, IS_NULLABLE AS is_nullable, DATETIME_PRECISION AS `precision`, COLUMN_DEFAULT AS `default` FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", TableName(config, "go_migrations")).Scan(&rows).Error; err != nil {
		return err
	}
	want := []struct {
		typ, nullable string
		precision     int
	}{{"bigint unsigned", "NO", 0}, {"varchar(191)", "NO", 0}, {"bigint unsigned", "NO", 0}, {"timestamp(6)", "NO", 6}, {"timestamp(6)", "YES", 6}, {"varchar(191)", "YES", 0}}
	if len(rows) != len(want) {
		return fmt.Errorf("go_migrations schema mismatch: got %d columns", len(rows))
	}
	wantNames := []string{"sequence", "migration_id", "revision", "start_time", "end_time", "adopted_from"}
	for i, row := range rows {
		w := want[i]
		if row.Name != wantNames[i] || strings.ToLower(row.Type) != w.typ || row.IsNullable != w.nullable || precisionValue(row.Precision) != w.precision || row.Default.Valid {
			return fmt.Errorf("go_migrations schema mismatch at %s", row.Name)
		}
	}
	type indexColumn struct {
		Name, Column string
		NonUnique    int
	}
	var indexes []indexColumn
	if err := db.Raw("SELECT INDEX_NAME AS name, COLUMN_NAME AS `column`, NON_UNIQUE AS non_unique FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY INDEX_NAME, SEQ_IN_INDEX", TableName(config, "go_migrations")).Scan(&indexes).Error; err != nil {
		return err
	}
	if len(indexes) != 2 || indexes[0].Name != "PRIMARY" || indexes[0].Column != "sequence" || indexes[0].NonUnique != 0 || indexes[1].Name != "uq_go_migrations_id" || indexes[1].Column != "migration_id" || indexes[1].NonUnique != 0 {
		return fmt.Errorf("go_migrations indexes mismatch")
	}
	return nil
}

func precisionValue(value sql.NullInt64) int {
	if !value.Valid {
		return 0
	}
	return int(value.Int64)
}

func InsertPendingLocalMigration(db *gorm.DB, config *conf.Configuration, m LocalMigration, adoptedFrom *string) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	now := time.Now()
	return db.Table(TableName(config, "go_migrations")).Create(&LocalMigrationRecord{Sequence: m.Sequence, MigrationID: m.ID, Revision: m.Revision, StartTime: now, AdoptedFrom: adoptedFrom}).Error
}

func CompleteLocalMigration(db *gorm.DB, config *conf.Configuration, m LocalMigration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	now := time.Now()
	result := db.Table(TableName(config, "go_migrations")).Where("sequence = ? AND migration_id = ? AND revision = ? AND end_time IS NULL", m.Sequence, m.ID, m.Revision).Update("end_time", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("local migration %s completion identity/revision/pending mismatch", m.ID)
	}
	return nil
}
