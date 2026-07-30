package crud_helper

import (
	"database/sql"
	"go-build-admin/app/admin/model"
	crudmodel "go-build-admin/app/admin/model/crud"
	"testing"
)

func alterTestColumn(name, columnType, nullable string, def interface{}, extra, comment string) model.Column {
	column := model.Column{COLUMN_NAME: name, COLUMN_TYPE: columnType, IS_NULLABLE: nullable, EXTRA: extra, COLUMN_COMMENT: comment}
	switch v := def.(type) {
	case string:
		column.COLUMN_DEFAULT = sql.NullString{Valid: true, String: v}
	}
	return column
}

func TestDeriveAlterChangesSkipsInSyncColumns(t *testing.T) {
	columns := []model.Column{
		alterTestColumn("id", "bigint", "NO", nil, "auto_increment", "主键"),
		alterTestColumn("code", "varchar(20)", "NO", nil, "", "货币代码"),
		alterTestColumn("rate", "decimal(20,8)", "NO", "1.00000000", "", "汇率"),
		alterTestColumn("status", "tinyint", "NO", "1", "", "状态:0=禁用,1=启用"),
	}
	fields := []crudmodel.Field{
		{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, Null: false, DefaultType: "NONE", Comment: "主键"},
		{Name: "code", Type: "varchar", Length: 20, Null: false, DefaultType: "NONE", Comment: "货币代码"},
		{Name: "rate", Type: "decimal", Length: 20, Precision: 8, Null: false, DefaultType: "INPUT", Default: "1", Comment: "汇率"},
		{Name: "status", Type: "tinyint", Null: false, DefaultType: "INPUT", Default: "1", Comment: "状态:0=禁用,1=启用"},
	}
	if changes := deriveAlterChanges(columns, fields); len(changes) != 0 {
		t.Fatalf("in-sync table must produce no changes: %+v", changes)
	}
}

func TestDeriveAlterChangesDetectsRealDrift(t *testing.T) {
	base := []model.Column{
		alterTestColumn("code", "varchar(20)", "NO", nil, "", "货币代码"),
		alterTestColumn("rate", "decimal(20,8)", "NO", "1.00000000", "", "汇率"),
		alterTestColumn("status", "tinyint(1)", "NO", "1", "", "状态:0=禁用,1=启用"),
	}
	cases := []struct {
		name  string
		field crudmodel.Field
	}{
		{"length drift", crudmodel.Field{Name: "code", Type: "varchar", Length: 50, Null: false, DefaultType: "NONE", Comment: "货币代码"}},
		{"comment drift", crudmodel.Field{Name: "code", Type: "varchar", Length: 20, Null: false, DefaultType: "NONE", Comment: "新注释"}},
		{"nullable drift", crudmodel.Field{Name: "code", Type: "varchar", Length: 20, Null: true, DefaultType: "NULL", Comment: "货币代码"}},
		{"default drift", crudmodel.Field{Name: "rate", Type: "decimal", Length: 20, Precision: 8, Null: false, DefaultType: "INPUT", Default: "2", Comment: "汇率"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changes := deriveAlterChanges(base, []crudmodel.Field{tc.field})
			if len(changes) != 1 || changes[0].Type != "change-field-attr" {
				t.Fatalf("expected one change-field-attr: %+v", changes)
			}
		})
	}
	// tinyint(1) 与 tinyint 在 MySQL 8 显示宽度归一化后视为一致
	changes := deriveAlterChanges(base, []crudmodel.Field{{Name: "status", Type: "tinyint", Length: 1, Null: false, DefaultType: "INPUT", Default: "1", Comment: "状态:0=禁用,1=启用"}})
	if len(changes) != 0 {
		t.Fatalf("int display width must be normalized: %+v", changes)
	}
	// enum 逗号空格差异视为一致
	enumColumns := []model.Column{alterTestColumn("status", "enum('pending', 'paid')", "NO", "pending", "", "状态")}
	enumChanges := deriveAlterChanges(enumColumns, []crudmodel.Field{{Name: "status", DataType: "enum('pending','paid')", Null: false, DefaultType: "INPUT", Default: "pending", Comment: "状态"}})
	if len(enumChanges) != 0 {
		t.Fatalf("enum spacing must be normalized: %+v", enumChanges)
	}
}

func TestDeriveAlterChangesAddFieldOnlyForMissingColumn(t *testing.T) {
	columns := []model.Column{alterTestColumn("id", "bigint", "NO", nil, "auto_increment", "ID")}
	fields := []crudmodel.Field{
		{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, DefaultType: "NONE", Comment: "ID"},
		{Name: "title", Type: "varchar", Length: 50, Null: false, DefaultType: "NONE", Comment: "标题"},
	}
	changes := deriveAlterChanges(columns, fields)
	if len(changes) != 1 || changes[0].Type != "add-field" || changes[0].NewName != "title" {
		t.Fatalf("expected single add-field for title: %+v", changes)
	}
}

func TestAlterDiffRiskClasses(t *testing.T) {
	actual := []model.Column{
		alterTestColumn("id", "bigint unsigned", "NO", nil, "auto_increment", "ID"),
		alterTestColumn("name", "varchar(20)", "NO", "", "", "名称"),
		alterTestColumn("amount", "decimal(10,2)", "NO", "1.00000000", "", "金额"),
		alterTestColumn("status", "tinyint unsigned", "NO", "1", "", "状态"),
	}
	fields := []crudmodel.Field{
		{Name: "id", Type: "bigint", Unsigned: true, PrimaryKey: true, AutoIncrement: true, Comment: "ID"},
		{Name: "name", Type: "varchar", Length: 20, DefaultType: "EMPTY STRING", Comment: "名称"},
		{Name: "amount", Type: "decimal", Length: 10, Precision: 2, DefaultType: "INPUT", Default: "1", Comment: "金额"},
		{Name: "status", Type: "tinyint", Unsigned: true, DefaultType: "INPUT", Default: "1", Comment: "状态"},
	}

	cases := []struct {
		name   string
		field  crudmodel.Field
		column model.Column
		class  DiffClass
	}{
		{"nullable addition", crudmodel.Field{Name: "note", Type: "varchar", Length: 20, Null: true}, model.Column{}, DiffSafeAuto},
		{"defaulted addition", crudmodel.Field{Name: "enabled", Type: "tinyint", DefaultType: "INPUT", Default: "1"}, model.Column{}, DiffSafeAuto},
		{"comment only", fields[1], alterTestColumn("name", "varchar(20)", "NO", "", "", "旧名称"), DiffSafeAuto},
		{"type widening", crudmodel.Field{Name: "name", Type: "varchar", Length: 40, DefaultType: "EMPTY STRING", Comment: "名称"}, actual[1], DiffRequiresApproval},
		{"default change", crudmodel.Field{Name: "amount", Type: "decimal", Length: 10, Precision: 2, DefaultType: "INPUT", Default: "2", Comment: "金额"}, actual[2], DiffRequiresApproval},
		{"unsigned flip", crudmodel.Field{Name: "status", Type: "tinyint", DefaultType: "INPUT", Default: "1", Comment: "状态"}, actual[3], DiffRejected},
		{"varchar narrowing", crudmodel.Field{Name: "name", Type: "varchar", Length: 10, DefaultType: "EMPTY STRING", Comment: "名称"}, actual[1], DiffRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.column.COLUMN_NAME == "" {
				diffs := deriveAlterDiff(nil, []crudmodel.Field{tc.field})
				if len(diffs) != 1 || diffs[0].Class != tc.class {
					t.Fatalf("diffs = %+v", diffs)
				}
				return
			}
			diffs := deriveAlterDiff([]model.Column{tc.column}, []crudmodel.Field{tc.field})
			if len(diffs) != 1 || diffs[0].Class != tc.class {
				t.Fatalf("diffs = %+v", diffs)
			}
		})
	}

	primaryDrift := deriveAlterDiff(actual, []crudmodel.Field{{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, Comment: "ID"}})
	if len(primaryDrift) != 1 || primaryDrift[0].Class != DiffRejected {
		t.Fatalf("primary drift = %+v", primaryDrift)
	}
}

func TestAlterDiffRejectsNullableDownAndEnumRemoval(t *testing.T) {
	nullable := deriveAlterDiff([]model.Column{alterTestColumn("note", "varchar(20)", "YES", nil, "", "备注")}, []crudmodel.Field{{Name: "note", Type: "varchar", Length: 20, Null: false, Comment: "备注"}})
	if len(nullable) != 1 || nullable[0].Class != DiffRejected {
		t.Fatalf("nullable down = %+v", nullable)
	}
	enum := deriveAlterDiff([]model.Column{alterTestColumn("state", "enum('a','b')", "NO", "a", "", "状态")}, []crudmodel.Field{{Name: "state", DataType: "enum('a')", DefaultType: "INPUT", Default: "a", Comment: "状态"}})
	if len(enum) != 1 || enum[0].Class != DiffRejected {
		t.Fatalf("enum removal = %+v", enum)
	}
	longText := deriveAlterDiff(nil, []crudmodel.Field{{Name: "body", Type: "longtext", DefaultType: "INPUT", Default: "body"}})
	if len(longText) != 1 || longText[0].Class != DiffRejected {
		t.Fatalf("illegal longtext default = %+v", longText)
	}
}

func TestDefaultNormalizationPreservesLargeIntegersAndStrings(t *testing.T) {
	large := crudmodel.Field{Name: "id", Type: "bigint", DefaultType: "INPUT", Default: "9007199254740993"}
	if !defaultsMatchColumn(large, sql.NullString{Valid: true, String: "9007199254740993"}) {
		t.Fatal("large integer default was not compared exactly")
	}
	decimal := crudmodel.Field{Name: "rate", Type: "decimal", DefaultType: "INPUT", Default: "1"}
	if !defaultsMatchColumn(decimal, sql.NullString{Valid: true, String: "1.00000000"}) {
		t.Fatal("decimal default was not normalized exactly")
	}
	text := crudmodel.Field{Name: "value", Type: "varchar", DefaultType: "INPUT", Default: " 001 "}
	if defaultsMatchColumn(text, sql.NullString{Valid: true, String: "001"}) {
		t.Fatal("string default was numerically normalized")
	}
}

func TestUnmanagedColumnAttributesDoNotChangeModeledMatch(t *testing.T) {
	column := alterTestColumn("name", "varchar(20)", "NO", "", "on update CURRENT_TIMESTAMP", "名称")
	column.CHARACTER_SET_NAME = "utf8mb4"
	column.COLLATION_NAME = "utf8mb4_bin"
	column.GENERATION_EXPRESSION = "upper(name)"
	field := crudmodel.Field{Name: "name", Type: "varchar", Length: 20, DefaultType: "EMPTY STRING", Comment: "名称"}
	if !specFieldMatchesColumn(field, column) {
		t.Fatal("unmanaged attributes created a modeled diff")
	}
	attributes := unmanagedColumnAttributes(column)
	if len(attributes) != 4 {
		t.Fatalf("unmanaged attributes = %v", attributes)
	}
}
