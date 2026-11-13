package crudmodel

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBoolOrStringUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  BoolOrString
	}{
		{"bool true", `{"select-multi": true}`, "1"},
		{"bool false", `{"select-multi": false}`, ""},
		{"string 1", `{"select-multi": "1"}`, "1"},
		{"empty string", `{"select-multi": ""}`, ""},
		{"absent", `{}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var f FormAttr
			if err := json.Unmarshal([]byte(tc.input), &f); err != nil {
				t.Fatalf("unmarshal %s: %v", tc.input, err)
			}
			if f.SelectMulti != tc.want {
				t.Fatalf("got %q, want %q", f.SelectMulti, tc.want)
			}
		})
	}
}

func TestBoolOrStringUnmarshalInvalid(t *testing.T) {
	var f FormAttr
	if err := json.Unmarshal([]byte(`{"select-multi": 123}`), &f); err == nil {
		t.Fatal("expected error for numeric select-multi")
	}
}

func TestTableAttrUnmarshalDesignerSearchInputAttr(t *testing.T) {
	var payload struct {
		Table TableAttr `json:"table"`
	}
	for _, data := range []string{
		`{"table":{"comSearchInputAttr":""}}`,
		`{"table":{"comSearchInputAttr":"size=large\nplaceholder=Search here"}}`,
	} {
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			t.Fatal(err)
		}
	}
	if payload.Table.ComSearchInputAttr["size"] != "large" || payload.Table.ComSearchInputAttr["placeholder"] != "Search here" {
		t.Fatalf("designer attrs = %#v", payload.Table.ComSearchInputAttr)
	}
}

func TestTableAttrUnmarshalObjectSearchInputAttr(t *testing.T) {
	var attrs TableAttr
	if err := json.Unmarshal([]byte(`{"comSearchInputAttr":{"size":"large","disabled":false}}`), &attrs); err != nil {
		t.Fatal(err)
	}
	if attrs.ComSearchInputAttr["size"] != "large" || attrs.ComSearchInputAttr["disabled"] != false {
		t.Fatalf("object attrs = %#v", attrs.ComSearchInputAttr)
	}
}

func TestJSONColumnsSQLiteRoundTrip(t *testing.T) {
	type jsonRow struct {
		ID     int32       `gorm:"primaryKey"`
		Table  JSON_TABLE  `gorm:"column:table_json;type:text"`
		Fields JSON_FIELDS `gorm:"column:fields_json;type:text"`
	}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:crudmodel-json-roundtrip-%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err = db.AutoMigrate(&jsonRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	table := JSON_TABLE{
		Name:         "ops_banner",
		ColumnFields: []string{"title", "weigh"},
	}
	fields := JSON_FIELDS{
		{Name: "title", Type: "string", Title: "标题"},
		{Name: "weigh", Type: "int", Title: "权重"},
	}
	if err = db.Create(&jsonRow{ID: 1, Table: table, Fields: fields}).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var got jsonRow
	if err = db.First(&got, 1).Error; err != nil {
		t.Fatalf("first: %v", err)
	}
	if got.Table.Name != table.Name || len(got.Table.ColumnFields) != 2 {
		t.Fatalf("table round trip = %#v", got.Table)
	}
	if len(got.Fields) != 2 || got.Fields[0].Name != "title" || got.Fields[1].Comment != "" {
		t.Fatalf("fields round trip = %#v", got.Fields)
	}

	// 空载体写回文本列后仍能读回，保证空表 spec 的往返不依赖 NULL。
	if err = db.Create(&jsonRow{ID: 2, Table: JSON_TABLE{}, Fields: JSON_FIELDS{}}).Error; err != nil {
		t.Fatalf("create empty: %v", err)
	}
	got = jsonRow{}
	if err = db.First(&got, 2).Error; err != nil {
		t.Fatalf("first empty: %v", err)
	}
	if got.Table.Name != "" || len(got.Fields) != 0 {
		t.Fatalf("empty round trip = %#v / %#v", got.Table, got.Fields)
	}
}

func TestJSONColumnsScanValues(t *testing.T) {
	cases := []struct {
		name          string
		value         interface{}
		wantTableErr  string
		wantFieldsErr string
	}{
		{name: "nil", value: nil, wantTableErr: "unexpected end of JSON input", wantFieldsErr: "unexpected end of JSON input"},
		{name: "empty string", value: "", wantTableErr: "unexpected end of JSON input", wantFieldsErr: "unexpected end of JSON input"},
		{name: "empty object", value: []byte(`{}`), wantFieldsErr: "cannot unmarshal object"},
		{name: "empty array", value: `[]`, wantTableErr: "cannot unmarshal array"},
		{name: "invalid json", value: `{`, wantTableErr: "unexpected end of JSON input", wantFieldsErr: "unexpected end of JSON input"},
		{name: "unsupported type", value: 42, wantTableErr: "cannot scan int as JSON bytes", wantFieldsErr: "cannot scan int as JSON bytes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var table JSON_TABLE
			assertScanErr(t, "table", table.Scan(tc.value), tc.wantTableErr)

			var fields JSON_FIELDS
			assertScanErr(t, "fields", fields.Scan(tc.value), tc.wantFieldsErr)
		})
	}
}

func assertScanErr(t *testing.T, label string, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("%s scan: %v", label, err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("%s scan error = %v, want %q", label, err, want)
	}
}

func TestJSONColumnsValue(t *testing.T) {
	tableValue, err := JSON_TABLE{Name: "ops_banner"}.Value()
	if err != nil {
		t.Fatalf("table value: %v", err)
	}
	if text, ok := tableValue.(string); !ok || !strings.Contains(text, `"name":"ops_banner"`) {
		t.Fatalf("table value = %#v", tableValue)
	}
	fieldsValue, err := JSON_FIELDS{{Name: "title"}}.Value()
	if err != nil {
		t.Fatalf("fields value: %v", err)
	}
	if text, ok := fieldsValue.(string); !ok || !strings.Contains(text, `"name":"title"`) {
		t.Fatalf("fields value = %#v", fieldsValue)
	}
}
