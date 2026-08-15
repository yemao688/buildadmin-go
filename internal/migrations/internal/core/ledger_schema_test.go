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
	expected := []ledgerColumnSpec{{
		Name: "start_time", Type: "timestamp(6)", Nullable: "NO",
		Precision: 6, CheckPrecision: true, AllowCurrentTimestampDefault: true,
	}}
	// 无默认（ON 模式旧表形态）：通过
	actual := []ledgerColumn{{
		Name: "start_time", Type: "timestamp(6)", Nullable: "NO",
		Precision: sql.NullInt64{Int64: 6, Valid: true},
	}}
	if mismatch := compareLedgerColumns(actual, expected); mismatch != nil {
		t.Fatalf("tracked ledger columns mismatch: %+v", mismatch)
	}

	// 显式 DEFAULT CURRENT_TIMESTAMP(6)（新 DDL，OFF/ON 两模式一致）：通过
	actual[0].Default = sql.NullString{String: "CURRENT_TIMESTAMP(6)", Valid: true}
	if mismatch := compareLedgerColumns(actual, expected); mismatch != nil {
		t.Fatalf("tracked ledger CURRENT_TIMESTAMP default should pass: %+v", mismatch)
	}

	// 其它默认值：拒绝
	actual[0].Default = sql.NullString{String: "1970-01-01 00:00:00", Valid: true}
	if mismatch := compareLedgerColumns(actual, expected); mismatch == nil || mismatch.columnName != "start_time" {
		t.Fatalf("tracked ledger foreign default should mismatch: %+v", mismatch)
	}
}
