package crud_helper

import (
	"database/sql"
	"go-build-admin/app/admin/model"
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
	fields := []model.Field{
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
		field model.Field
	}{
		{"length drift", model.Field{Name: "code", Type: "varchar", Length: 50, Null: false, DefaultType: "NONE", Comment: "货币代码"}},
		{"comment drift", model.Field{Name: "code", Type: "varchar", Length: 20, Null: false, DefaultType: "NONE", Comment: "新注释"}},
		{"nullable drift", model.Field{Name: "code", Type: "varchar", Length: 20, Null: true, DefaultType: "NULL", Comment: "货币代码"}},
		{"default drift", model.Field{Name: "rate", Type: "decimal", Length: 20, Precision: 8, Null: false, DefaultType: "INPUT", Default: "2", Comment: "汇率"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changes := deriveAlterChanges(base, []model.Field{tc.field})
			if len(changes) != 1 || changes[0].Type != "change-field-attr" {
				t.Fatalf("expected one change-field-attr: %+v", changes)
			}
		})
	}
	// tinyint(1) 与 tinyint 在 MySQL 8 显示宽度归一化后视为一致
	changes := deriveAlterChanges(base, []model.Field{{Name: "status", Type: "tinyint", Length: 1, Null: false, DefaultType: "INPUT", Default: "1", Comment: "状态:0=禁用,1=启用"}})
	if len(changes) != 0 {
		t.Fatalf("int display width must be normalized: %+v", changes)
	}
	// enum 逗号空格差异视为一致
	enumColumns := []model.Column{alterTestColumn("status", "enum('pending', 'paid')", "NO", "pending", "", "状态")}
	enumChanges := deriveAlterChanges(enumColumns, []model.Field{{Name: "status", DataType: "enum('pending','paid')", Null: false, DefaultType: "INPUT", Default: "pending", Comment: "状态"}})
	if len(enumChanges) != 0 {
		t.Fatalf("enum spacing must be normalized: %+v", enumChanges)
	}
}

func TestDeriveAlterChangesAddFieldOnlyForMissingColumn(t *testing.T) {
	columns := []model.Column{alterTestColumn("id", "bigint", "NO", nil, "auto_increment", "ID")}
	fields := []model.Field{
		{Name: "id", Type: "bigint", PrimaryKey: true, AutoIncrement: true, DefaultType: "NONE", Comment: "ID"},
		{Name: "title", Type: "varchar", Length: 50, Null: false, DefaultType: "NONE", Comment: "标题"},
	}
	changes := deriveAlterChanges(columns, fields)
	if len(changes) != 1 || changes[0].Type != "add-field" || changes[0].NewName != "title" {
		t.Fatalf("expected single add-field for title: %+v", changes)
	}
}
