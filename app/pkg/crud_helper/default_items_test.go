package crud_helper

import (
	"database/sql"
	"go-build-admin/app/admin/model"
	"testing"
)

func TestGetFieldDefault(t *testing.T) {
	cases := []struct {
		name  string
		field model.Field
		want  string
	}{
		{
			name:  "switch INPUT 1",
			field: model.Field{Name: "status", DesignType: "switch", Default: "1", DefaultType: "INPUT"},
			want:  "status:'1'",
		},
		{
			name:  "switch INPUT 0 为无意义默认值",
			field: model.Field{Name: "status", DesignType: "switch", Default: "0", DefaultType: "INPUT"},
			want:  "",
		},
		{
			name:  "float NULL 不生成",
			field: model.Field{Name: "float", DesignType: "number", DefaultType: "NULL"},
			want:  "",
		},
		{
			name:  "weigh(映射为 number) NULL 不生成",
			field: model.Field{Name: "weigh", DesignType: "number", DefaultType: "NULL"},
			want:  "",
		},
		{
			name:  "number INPUT 非零输出原始数值",
			field: model.Field{Name: "weigh", DesignType: "number", Default: "5", DefaultType: "INPUT"},
			want:  "weigh:5",
		},
		{
			name:  "float INPUT 小数输出原始数值",
			field: model.Field{Name: "float", DesignType: "number", Default: "1.50", DefaultType: "INPUT"},
			want:  "float:1.5",
		},
		{
			name:  "number INPUT 0 为无意义默认值",
			field: model.Field{Name: "weigh", DesignType: "number", Default: "0", DefaultType: "INPUT"},
			want:  "",
		},
		{
			name:  "checkbox INPUT 多值拆数组",
			field: model.Field{Name: "checkbox", DesignType: "checkbox", Default: "opt0,opt1", DefaultType: "INPUT"},
			want:  "checkbox:['opt0', 'opt1']",
		},
		{
			name:  "string INPUT 引号包裹",
			field: model.Field{Name: "string", DesignType: "string", Default: "hello", DefaultType: "INPUT"},
			want:  "string:'hello'",
		},
		{
			name:  "string 含单引号改用双引号",
			field: model.Field{Name: "string", DesignType: "string", Default: "it's", DefaultType: "INPUT"},
			want:  `string:"it's"`,
		},
		{
			name:  "NONE 不生成",
			field: model.Field{Name: "string", DesignType: "string", DefaultType: "NONE"},
			want:  "",
		},
		{
			name:  "array 固定空数组",
			field: model.Field{Name: "arr", DesignType: "array"},
			want:  "arr: []",
		},
		{
			name:  "editor 无默认值输出空字符串",
			field: model.Field{Name: "content", DesignType: "editor"},
			want:  "content:''",
		},
		{
			name:  "remoteSelect INPUT 0 为无意义默认值",
			field: model.Field{Name: "ba_admin_id", DesignType: "remoteSelect", Default: "0", DefaultType: "INPUT"},
			want:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := getFieldDefault(tc.field); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseTableColumnsDefaultType(t *testing.T) {
	columns := []model.Column{
		{COLUMN_NAME: "a", DATA_TYPE: "int", COLUMN_TYPE: "int(11)", IS_NULLABLE: "YES"},
		{COLUMN_NAME: "b", DATA_TYPE: "varchar", COLUMN_TYPE: "varchar(255)", IS_NULLABLE: "NO", COLUMN_DEFAULT: sql.NullString{Valid: true, String: ""}},
		{COLUMN_NAME: "c", DATA_TYPE: "int", COLUMN_TYPE: "int(11)", IS_NULLABLE: "NO"},
		{COLUMN_NAME: "d", DATA_TYPE: "tinyint", COLUMN_TYPE: "tinyint(1)", IS_NULLABLE: "NO", COLUMN_DEFAULT: sql.NullString{Valid: true, String: "1"}},
		{COLUMN_NAME: "e", DATA_TYPE: "int", COLUMN_TYPE: "int(11)", IS_NULLABLE: "YES", COLUMN_DEFAULT: sql.NullString{Valid: true, String: "3"}},
	}
	fields := ParseTableColumns(columns, false)
	byName := map[string]model.Field{}
	for _, f := range fields {
		byName[f.Name] = f
	}

	if byName["a"].DefaultType != "NULL" {
		t.Fatalf("a: got %q", byName["a"].DefaultType)
	}
	if byName["b"].DefaultType != "EMPTY STRING" {
		t.Fatalf("b: got %q", byName["b"].DefaultType)
	}
	if byName["c"].DefaultType != "NONE" {
		t.Fatalf("c: got %q", byName["c"].DefaultType)
	}
	if byName["d"].DefaultType != "INPUT" || byName["d"].Default != "1" {
		t.Fatalf("d: got %q %q", byName["d"].DefaultType, byName["d"].Default)
	}
	if byName["e"].DefaultType != "INPUT" || byName["e"].Default != "3" {
		t.Fatalf("e: got %q %q", byName["e"].DefaultType, byName["e"].Default)
	}
}
