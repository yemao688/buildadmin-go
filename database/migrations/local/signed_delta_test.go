package local

import (
	"testing"

	"go-build-admin/database/migrations/internal/core"
)

func TestIsSignedDeltaColumn(t *testing.T) {
	zero := func(value string) *string { return &value }
	tests := []struct {
		name string
		def  core.MigrationColumn
		want bool
	}{
		{name: "int", def: core.MigrationColumn{ColumnType: "int", Nullable: "NO", Default: zero("0")}, want: true},
		{name: "int with precision", def: core.MigrationColumn{ColumnType: "int(11)", Nullable: "NO", Default: zero("0")}, want: true},
		{name: "decimal with precision and scale", def: core.MigrationColumn{ColumnType: "decimal(10,2)", Nullable: "NO", Default: zero("0.00")}, want: true},
		{name: "double", def: core.MigrationColumn{ColumnType: "double", Nullable: "NO", Default: zero("0")}, want: true},
		{name: "float", def: core.MigrationColumn{ColumnType: "float", Nullable: "NO", Default: zero("0")}, want: true},
		{name: "decimal", def: core.MigrationColumn{ColumnType: "decimal", Nullable: "NO", Default: zero("0")}, want: true},
		{name: "varchar", def: core.MigrationColumn{ColumnType: "varchar(20)", Nullable: "NO", Default: zero("0")}, want: false},
		{name: "nullable decimal", def: core.MigrationColumn{ColumnType: "decimal(10,2)", Nullable: "YES", Default: zero("0.00")}, want: false},
		{name: "decimal without default", def: core.MigrationColumn{ColumnType: "decimal(10,2)", Nullable: "NO"}, want: false},
		{name: "nonzero decimal default", def: core.MigrationColumn{ColumnType: "decimal(10,2)", Nullable: "NO", Default: zero("1.50")}, want: false},
		{name: "text", def: core.MigrationColumn{ColumnType: "text", Nullable: "NO", Default: zero("0")}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isSignedDeltaColumn(test.def); got != test.want {
				t.Fatalf("isSignedDeltaColumn(%+v) = %v, want %v", test.def, got, test.want)
			}
		})
	}
}
