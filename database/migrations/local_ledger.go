package migrations

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

// OfficialKey is the immutable identity used by the upstream ledger.
type OfficialKey struct {
	Version int64
	Name    string
}

type OfficialMigration struct {
	Key OfficialKey
	// Source identifies the upstream source document or release record.
	Source string
	Up     MigrationFn
}

type LocalMigration struct {
	Sequence          uint64
	ID                string
	Revision          uint64
	RequiresOfficial  []OfficialKey
	Up                MigrationFn
	VerifySchema      func(*gorm.DB, *conf.Configuration) error
	VerifyUpgradeData func(*gorm.DB, *conf.Configuration) error
	PostSeedVerify    func(*gorm.DB, *conf.Configuration) error
	LegacyAliases     []OfficialKey
}

type LocalMigrationRecord struct {
	Sequence    uint64     `gorm:"column:sequence"`
	MigrationID string     `gorm:"column:migration_id"`
	Revision    uint64     `gorm:"column:revision"`
	StartTime   time.Time  `gorm:"column:start_time"`
	EndTime     *time.Time `gorm:"column:end_time"`
	AdoptedFrom *string    `gorm:"column:adopted_from"`
}

func (LocalMigrationRecord) TableName() string { return "go_migrations" }

func ValidateOfficialMigrations(list []OfficialMigration) error {
	var previous int64
	seen := map[int64]string{}
	for i, m := range list {
		if m.Key.Version <= previous || m.Key.Version == 0 || strings.TrimSpace(m.Key.Name) == "" || strings.TrimSpace(m.Source) == "" || m.Up == nil {
			return fmt.Errorf("invalid official migration at index %d", i)
		}
		if old, ok := seen[m.Key.Version]; ok && old != m.Key.Name {
			return fmt.Errorf("official version %d name collision", m.Key.Version)
		}
		seen[m.Key.Version] = m.Key.Name
		previous = m.Key.Version
	}
	return nil
}

func ValidateLocalMigrations(list []LocalMigration, official []OfficialMigration) error {
	if err := ValidateOfficialMigrations(official); err != nil {
		return err
	}
	officialKeys := map[OfficialKey]bool{}
	for _, m := range official {
		officialKeys[m.Key] = true
	}
	seenID, seenSeq := map[string]bool{}, map[uint64]bool{}
	var previousSequence uint64
	for i, m := range list {
		if m.Sequence == 0 || m.Sequence <= previousSequence || strings.TrimSpace(m.ID) == "" || m.Revision == 0 || m.Up == nil || seenID[m.ID] || seenSeq[m.Sequence] {
			return fmt.Errorf("invalid local migration at index %d", i)
		}
		for _, key := range m.RequiresOfficial {
			if !officialKeys[key] {
				return fmt.Errorf("local migration %s requires unknown official migration %d/%s", m.ID, key.Version, key.Name)
			}
		}
		seenID[m.ID], seenSeq[m.Sequence] = true, true
		previousSequence = m.Sequence
	}
	return nil
}

func localLedgerTable(config *conf.Configuration) string {
	return quoteIdentifier(tableName(config, "go_migrations"))
}

// BootstrapLocalLedger deliberately uses fixed DDL. It never calls AutoMigrate.
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
	if err := db.Raw("SELECT ENGINE FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", tableName(config, "go_migrations")).Scan(&engine).Error; err != nil {
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
	if err := db.Raw("SELECT COLUMN_NAME AS name, COLUMN_TYPE AS type, IS_NULLABLE AS is_nullable, DATETIME_PRECISION AS `precision`, COLUMN_DEFAULT AS `default` FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", tableName(config, "go_migrations")).Scan(&rows).Error; err != nil {
		return err
	}
	type expected struct {
		typ, nullable string
		precision     int
	}
	want := []expected{{"bigint unsigned", "NO", 0}, {"varchar(191)", "NO", 0}, {"bigint unsigned", "NO", 0}, {"timestamp(6)", "NO", 6}, {"timestamp(6)", "YES", 6}, {"varchar(191)", "YES", 0}}
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
	if err := db.Raw("SELECT INDEX_NAME AS name, COLUMN_NAME AS `column`, NON_UNIQUE AS non_unique FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY INDEX_NAME, SEQ_IN_INDEX", tableName(config, "go_migrations")).Scan(&indexes).Error; err != nil {
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
	return db.Table(tableName(config, "go_migrations")).Create(&LocalMigrationRecord{Sequence: m.Sequence, MigrationID: m.ID, Revision: m.Revision, StartTime: now, AdoptedFrom: adoptedFrom}).Error
}

func CompleteLocalMigration(db *gorm.DB, config *conf.Configuration, m LocalMigration) error {
	if err := ValidatePrefix(config); err != nil {
		return err
	}
	now := time.Now()
	result := db.Table(tableName(config, "go_migrations")).Where("sequence = ? AND migration_id = ? AND revision = ? AND end_time IS NULL", m.Sequence, m.ID, m.Revision).Update("end_time", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("local migration %s completion identity/revision/pending mismatch", m.ID)
	}
	return nil
}
