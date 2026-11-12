package crudgen

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDDLFieldData_NullableSemantics(t *testing.T) {
	nullable, err := getDDlFieldData(crudmodel.Field{Name: "nickname", Type: "varchar", Length: 64, Null: true})
	require.NoError(t, err)
	assert.NotContains(t, nullable, "NOT NULL")

	notNullable, err := getDDlFieldData(crudmodel.Field{Name: "status", Type: "int", Null: false})
	require.NoError(t, err)
	assert.Contains(t, notNullable, "NOT NULL")
}

func TestGetDDLFieldDataDefaultTypes(t *testing.T) {
	for _, tc := range []struct {
		name, defaultType, value, want string
	}{
		{"none", "NONE", "ignored", "NOT NULL"},
		{"null", "NULL", "", "DEFAULT NULL"},
		{"empty", "EMPTY STRING", "", "DEFAULT ''"},
		{"input", "INPUT", "", "DEFAULT ''"},
		{"legacy_null", "", "null", "DEFAULT NULL"},
		{"legacy_empty", "", "empty string", "DEFAULT ''"},
		{"stale_none", "NONE", "stale", "NOT NULL"},
		{"stale_null", "NULL", "stale", "DEFAULT NULL"},
		{"stale_empty", "EMPTY STRING", "stale", "DEFAULT ''"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getDDlFieldData(crudmodel.Field{Name: tc.name, Type: "varchar", Length: 32, DefaultType: tc.defaultType, Default: tc.value})
			require.NoError(t, err)
			assert.Contains(t, got, tc.want)
		})
	}
}

func TestGetDDLFieldDataNullDefaultIsNullable(t *testing.T) {
	got, err := getDDlFieldData(crudmodel.Field{Name: "value", Type: "varchar", DefaultType: "NULL"})
	require.NoError(t, err)
	assert.NotContains(t, got, "NOT NULL")
	assert.Contains(t, got, "DEFAULT NULL")
}

func TestGetDDLFieldDataNoDefaultFamilies(t *testing.T) {
	for dataType := range noDefaultValueTypes {
		t.Run(dataType, func(t *testing.T) {
			got, err := getDDlFieldData(crudmodel.Field{Name: "value", Type: dataType, DefaultType: "INPUT", Default: "x"})
			require.NoError(t, err)
			assert.NotContains(t, got, "DEFAULT")
		})
	}
}

func TestColumnPositionClause(t *testing.T) {
	cases := map[string]string{"": "", firstFieldSentinel: " FIRST", "image": " AFTER `image`"}
	for after, want := range cases {
		if got := columnPositionClause(after); got != want {
			t.Errorf("columnPositionClause(%q) = %q, want %q", after, got, want)
		}
	}
}
