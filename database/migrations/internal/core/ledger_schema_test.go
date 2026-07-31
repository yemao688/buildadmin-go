package core

import (
	"database/sql"
	"testing"
)

func TestCompareLedgerColumns(t *testing.T) {
	actual := []ledgerColumn{
		{Name: "version", Type: "bigint unsigned", Nullable: "NO"},
		{Name: "migration_name", Type: "varchar(100)", Nullable: "YES"},
	}
	expected := []ledgerColumnSpec{
		{Name: "version", Type: "bigint", Nullable: "NO", AcceptUnsigned: true},
		{Name: "migration_name", Type: "varchar(100)", Nullable: "YES"},
	}
	if mismatch := compareLedgerColumns(actual, expected); mismatch != nil {
		t.Fatalf("official ledger columns mismatch: %+v", mismatch)
	}
}

func TestCompareTrackedLedgerColumns(t *testing.T) {
	actual := []ledgerColumn{{
		Name: "start_time", Type: "timestamp(6)", Nullable: "NO",
		Precision: sql.NullInt64{Int64: 6, Valid: true},
	}}
	expected := []ledgerColumnSpec{{
		Name: "start_time", Type: "timestamp(6)", Nullable: "NO",
		Precision: 6, CheckPrecision: true, RequireNoDefault: true,
	}}
	if mismatch := compareLedgerColumns(actual, expected); mismatch != nil {
		t.Fatalf("tracked ledger columns mismatch: %+v", mismatch)
	}

	actual[0].Default = sql.NullString{String: "CURRENT_TIMESTAMP(6)", Valid: true}
	if mismatch := compareLedgerColumns(actual, expected); mismatch == nil || mismatch.columnName != "start_time" {
		t.Fatalf("tracked ledger default should mismatch: %+v", mismatch)
	}
}
