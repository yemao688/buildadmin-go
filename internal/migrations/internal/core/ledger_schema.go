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
	Extra     string         `gorm:"column:extra"`
}

type ledgerColumnSpec struct {
	Name                         string
	Type                         string
	Nullable                     string
	Precision                    int
	CheckPrecision               bool
	RequireNoDefault             bool
	AllowCurrentTimestampDefault bool
	AcceptUnsigned               bool
	RequireAutoIncrement         bool
	ExpectedDefault              *string
}

type ledgerColumnMismatch struct {
	actualCount int
	columnName  string
}

func queryLedgerColumns(db *gorm.DB, table string) ([]ledgerColumn, error) {
	var columns []ledgerColumn
	if err := db.Raw("SELECT COLUMN_NAME AS name, COLUMN_TYPE AS type, IS_NULLABLE AS nullable, DATETIME_PRECISION AS `precision`, COLUMN_DEFAULT AS `default`, EXTRA AS extra FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", table).Scan(&columns).Error; err != nil {
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
		if !ledgerColumnIdentityMatches(column, spec) ||
			!ledgerColumnPrecisionMatches(column, spec) ||
			!ledgerColumnDefaultMatches(column, spec) {
			return &ledgerColumnMismatch{columnName: column.Name}
		}
	}
	return nil
}

func ledgerColumnIdentityMatches(column ledgerColumn, spec ledgerColumnSpec) bool {
	typeMatches := strings.EqualFold(column.Type, spec.Type)
	if spec.AcceptUnsigned {
		typeMatches = typeMatches || strings.EqualFold(column.Type, spec.Type+" unsigned")
	}
	return column.Name == spec.Name && typeMatches && column.Nullable == spec.Nullable
}

func ledgerColumnPrecisionMatches(column ledgerColumn, spec ledgerColumnSpec) bool {
	if spec.CheckPrecision && precisionValue(column.Precision) != spec.Precision {
		return false
	}
	if spec.RequireAutoIncrement && !strings.Contains(strings.ToLower(column.Extra), "auto_increment") {
		return false
	}
	return true
}

func ledgerColumnDefaultMatches(column ledgerColumn, spec ledgerColumnSpec) bool {
	if spec.RequireNoDefault && column.Default.Valid {
		return false
	}
	if spec.AllowCurrentTimestampDefault && column.Default.Valid &&
		!strings.EqualFold(column.Default.String, "CURRENT_TIMESTAMP(6)") {
		return false
	}
	return spec.ExpectedDefault == nil || (column.Default.Valid && column.Default.String == *spec.ExpectedDefault)
}

func precisionValue(value sql.NullInt64) int {
	if !value.Valid {
		return 0
	}
	return int(value.Int64)
}
