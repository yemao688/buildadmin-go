package crud_helper

import (
	"testing"

	"go-build-admin/app/admin/model"
)

func TestTimestampTypeOverridesSplitCanonicalAndNonCanonical(t *testing.T) {
	fields := []model.Field{
		{Name: "create_time", Type: "bigint", DataType: "bigint", DesignType: "timestamp"},
		{Name: "end_time", Type: "bigint", DataType: "bigint", DesignType: "timestamp"},
		{Name: "created_at", Type: "bigint", DataType: "bigint", DesignType: "timestamp"},
	}
	analysed := make([]model.Field, len(fields))
	for i, field := range fields {
		analysed[i] = analyseField(field)
	}

	modelTypes := buildModelFieldTypeOverrides(fields)
	paramTypes := buildHandlerParamTypeOverrides(analysed)
	want := map[string]string{
		"create_time": "validate.FlexUnixTime",
		"end_time":    "validate.FlexFormattedUnixTime",
		"created_at":  "validate.FlexFormattedUnixTime",
	}
	for name, typeName := range want {
		if modelTypes[name] != typeName {
			t.Errorf("model %s override = %q, want %q", name, modelTypes[name], typeName)
		}
		if paramTypes[name] != typeName {
			t.Errorf("handler %s override = %q, want %q", name, paramTypes[name], typeName)
		}
	}
}

func TestTimestampTypeOverridesKeepSQLTimeAdapters(t *testing.T) {
	fields := []model.Field{
		{Name: "at", Type: "datetime", DataType: "datetime", DesignType: "datetime"},
		{Name: "created", Type: "bigint", DataType: "bigint", DesignType: "timestamp"},
		{Name: "day", Type: "date", DataType: "date", DesignType: "date"},
		{Name: "clock", Type: "time", DataType: "time", DesignType: "time"},
		{Name: "year", Type: "year", DataType: "year", DesignType: "year"},
	}
	got := buildModelFieldTypeOverrides(fields)
	for name, want := range map[string]string{
		"at":      "validate.FlexDateTime",
		"created": "validate.FlexFormattedUnixTime",
		"day":     "validate.FlexDate",
		"clock":   "validate.FlexClock",
		"year":    "validate.FlexYear",
	} {
		if got[name] != want {
			t.Errorf("%s override = %q, want %q", name, got[name], want)
		}
	}
}
