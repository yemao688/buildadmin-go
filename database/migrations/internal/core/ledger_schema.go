package core

import (
	"database/sql"
	"strings"

	"gorm.io/gorm"
)

type ledgerColumn struct {
	Name      string         `gorm:"column:name"`
	Type      string         `gorm:"column:type"`
	Nullable  string         `gorm:"column:nullable"`
	Precision sql.NullInt64  `gorm:"column:precision"`
	Default   sql.NullString `gorm:"column:default"`
}

type ledgerColumnSpec struct {
	Name             string
	Type             string
	Nullable         string
	Precision        int
	CheckPrecision   bool
	RequireNoDefault bool
	AcceptUnsigned   bool
}

type ledgerColumnMismatch struct {
	actualCount int
	columnName  string
}

func queryLedgerColumns(db *gorm.DB, table string) ([]ledgerColumn, error) {
	var columns []ledgerColumn
	if err := db.Raw("SELECT COLUMN_NAME AS name, COLUMN_TYPE AS type, IS_NULLABLE AS nullable, DATETIME_PRECISION AS `precision`, COLUMN_DEFAULT AS `default` FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", table).Scan(&columns).Error; err != nil {
		return nil, err
	}
	return columns, nil
}

func compareLedgerColumns(actual []ledgerColumn, expected []ledgerColumnSpec) *ledgerColumnMismatch {
	if len(actual) != len(expected) {
		return &ledgerColumnMismatch{actualCount: len(actual)}
	}
	for i, column := range actual {
		spec := expected[i]
		typeMatches := strings.EqualFold(column.Type, spec.Type)
		if spec.AcceptUnsigned {
			typeMatches = typeMatches || strings.EqualFold(column.Type, spec.Type+" unsigned")
		}
		if column.Name != spec.Name || !typeMatches || column.Nullable != spec.Nullable {
			return &ledgerColumnMismatch{columnName: column.Name}
		}
		if spec.CheckPrecision && precisionValue(column.Precision) != spec.Precision {
			return &ledgerColumnMismatch{columnName: column.Name}
		}
		if spec.RequireNoDefault && column.Default.Valid {
			return &ledgerColumnMismatch{columnName: column.Name}
		}
	}
	return nil
}

func precisionValue(value sql.NullInt64) int {
	if !value.Valid {
		return 0
	}
	return int(value.Int64)
}
