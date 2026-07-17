package core

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"go-build-admin/conf"
	"gorm.io/gorm"
)

var safePrefix = regexp.MustCompile(`^[A-Za-z0-9_]*$`)

func ValidatePrefix(config *conf.Configuration) error {
	if config == nil || !safePrefix.MatchString(config.Database.Prefix) {
		return fmt.Errorf("invalid database table prefix")
	}
	return nil
}

func TableName(config *conf.Configuration, logicalName string) string {
	return config.Database.Prefix + logicalName
}

func QuoteIdentifier(value string) string { return "`" + strings.ReplaceAll(value, "`", "``") + "`" }

func LegacyTableExists(db *gorm.DB, name string) (bool, error) {
	var count int64
	result := db.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", name).Scan(&count)
	return count > 0, result.Error
}

func LegacyColumnExists(db *gorm.DB, table, column string) (bool, error) {
	var count int64
	result := db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?", table, column).Scan(&count)
	return count > 0, result.Error
}

func TableExists(db *gorm.DB, name string) bool {
	ok, _ := LegacyTableExists(db, name)
	return ok
}

func ColumnExists(db *gorm.DB, table, column string) bool {
	ok, _ := LegacyColumnExists(db, table, column)
	return ok
}

func IndexExists(db *gorm.DB, table, index string) bool {
	var count int64
	result := db.Raw("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?", table, index).Scan(&count)
	return result.Error == nil && count > 0
}

func IndexFirstColumn(db *gorm.DB, table, index string) (string, error) {
	var column string
	err := db.Raw("SELECT column_name FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ? AND seq_in_index = 1", table, index).Scan(&column).Error
	return column, err
}

type MigrationColumn struct {
	ColumnType string  `gorm:"column:column_type"`
	Nullable   string  `gorm:"column:is_nullable"`
	Default    *string `gorm:"column:column_default"`
}

func MigrationColumnInfo(db *gorm.DB, table, column string) (MigrationColumn, bool, error) {
	var columnType, nullable string
	var defaultValue sql.NullString
	err := db.Raw("SELECT column_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?", table, column).Row().Scan(&columnType, &nullable, &defaultValue)
	if errors.Is(err, sql.ErrNoRows) {
		return MigrationColumn{}, false, nil
	}
	if err != nil {
		return MigrationColumn{}, false, err
	}
	var defaultPtr *string
	if defaultValue.Valid {
		value := defaultValue.String
		defaultPtr = &value
	}
	return MigrationColumn{ColumnType: columnType, Nullable: nullable, Default: defaultPtr}, true, nil
}

func MigrationIndexInfo(db *gorm.DB, table, index string) (bool, string, error) {
	var column string
	err := db.Raw("SELECT column_name FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=? AND seq_in_index=1", table, index).Row().Scan(&column)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, column, nil
}

type MigrationRecord struct {
	Version       int64      `gorm:"column:version"`
	MigrationName string     `gorm:"column:migration_name"`
	StartTime     time.Time  `gorm:"column:start_time"`
	EndTime       *time.Time `gorm:"column:end_time"`
	Breakpoint    bool       `gorm:"column:breakpoint"`
}
