package crudgen

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"strings"
	"testing"
)

// 实体由 spec 直接渲染：字段顺序、tag 顺序与默认值都必须稳定，否则离线渲染与
// 在线生成会不一致。
func TestBuildSpecEntityStruct(t *testing.T) {
	t.Parallel()
	table := crudmodel.Table{
		Name:    "ops_demo",
		Comment: "演示表",
		Indexes: []crudmodel.IndexSpec{
			{Name: "uk_code", Unique: true, Columns: []string{"code"}},
			{Name: "idx_code_name", Columns: []string{"code", "name"}},
		},
	}
	fields := []crudmodel.Field{
		{Name: "id", Type: "bigint", Unsigned: true, PrimaryKey: true, AutoIncrement: true, Comment: "ID"},
		{Name: "code", Type: "varchar", Length: 20, DefaultType: "EMPTY STRING", Comment: "编码"},
		{Name: "name", Type: "varchar", Length: 50, Null: true, Comment: "名称"},
		{Name: "admin_id", Type: "int", DefaultType: "NONE", Comment: "归属"},
		{Name: "last_login_ip", Type: "varchar", Length: 50, Comment: "登录IP"},
	}
	content, err := buildSpecEntityStruct(table, fields, "OpsDemo", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"// OpsDemo 演示管理",
		"ID int64 `gorm:\"column:id;type:bigint unsigned;primaryKey;autoIncrement:true;comment:ID\" json:\"id\"`",
		"Code string `gorm:\"column:code;default:'';type:varchar(20);not null;uniqueIndex:uk_code,priority:1;index:idx_code_name,priority:1;comment:编码\" json:\"code\"`",
		"Name string `gorm:\"column:name;type:varchar(50);index:idx_code_name,priority:2;comment:名称\" json:\"name\"`",
		"AdminID int32 `gorm:\"column:admin_id;type:int;not null;comment:归属\" json:\"admin_id\"`",
		"LastLoginIP string `gorm:\"column:last_login_ip;type:varchar(50);not null;comment:登录IP\" json:\"last_login_ip\"`",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("entity missing %q:\n%s", want, content)
		}
	}
	// nullable 字段不得输出 not null
	if strings.Contains(content, `column:name;type:varchar(50);not null`) {
		t.Fatalf("nullable field must not carry not null:\n%s", content)
	}
}

func TestSpecMenuName(t *testing.T) {
	t.Parallel()
	path := writeSpecTest(t, `name: ops_banner
fields:
  - name: id
    type: bigint
    primaryKey: true
`)
	name, err := specMenuName(path)
	if err != nil {
		t.Fatal(err)
	}
	if name != "ops/banner" {
		t.Fatalf("menu name = %q", name)
	}
}

// 实体字段类型与命名必须与旧 gorm.io/gen 产物一致：下游模块重新生成时
// float→float32、blob→[]byte、tinyint(1)→bool，缩写字段名走 gorm 规则。
func TestEntityGoTypeAndNameMatchGormGen(t *testing.T) {
	t.Parallel()
	cases := []struct {
		field    crudmodel.Field
		wantType string
		wantName string
	}{
		{crudmodel.Field{Name: "id", Type: "bigint"}, "int64", "ID"},
		{crudmodel.Field{Name: "admin_id", Type: "int"}, "int32", "AdminID"},
		{crudmodel.Field{Name: "ratio", Type: "float"}, "float32", "Ratio"},
		{crudmodel.Field{Name: "rate", Type: "decimal"}, "float64", "Rate"},
		{crudmodel.Field{Name: "payload", Type: "blob"}, "[]byte", "Payload"},
		{crudmodel.Field{Name: "summary", Type: "varchar"}, "string", "Summary"},
		{crudmodel.Field{Name: "flag", Type: "tinyint", Length: 1}, "bool", "Flag"},
		{crudmodel.Field{Name: "bits", Type: "bit"}, "[]uint8", "Bits"},
		{crudmodel.Field{Name: "html_content", Type: "varchar"}, "string", "HTMLContent"},
		{crudmodel.Field{Name: "cpu_load", Type: "int"}, "int32", "CPULoad"},
		{crudmodel.Field{Name: "user_ids", Type: "varchar"}, "string", "UserIDs"},
	}
	for _, tc := range cases {
		if got := entityGoType(tc.field); got != tc.wantType {
			t.Errorf("%s type = %s, want %s", tc.field.Name, got, tc.wantType)
		}
		if got := entityGoFieldName(tc.field.Name); got != tc.wantName {
			t.Errorf("%s name = %s, want %s", tc.field.Name, got, tc.wantName)
		}
	}
}
