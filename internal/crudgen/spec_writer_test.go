package crudgen

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/data_scope"
	"os"
	"path/filepath"
	"testing"
)

// 设计器的表设计写回 spec 后必须能被 CLI 原样读回，否则两条生成路径会再次
// 分叉。这里覆盖：字段属性、连字符 JSON 键 → camelCase YAML 键、索引、
// dataScope 与表单/列表字段。
func TestMarshalSpecRoundTripsThroughLoadSpec(t *testing.T) {
	t.Parallel()
	table := crudmodel.Table{
		Name:                 "ops_banner",
		Comment:              "轮播图表",
		GenerateRelativePath: "ops_banner",
		QuickSearchField:     []string{"title"},
		FormFields:           []string{"title", "user_id", "status"},
		ColumnFields:         []string{"title", "user_id", "status", "create_time"},
		DataScope:            &data_scope.Config{Mode: data_scope.ModeAuto},
		Indexes:              []crudmodel.IndexSpec{{Name: "uk_title", Unique: true, Columns: []string{"title"}}},
	}
	fields := []crudmodel.Field{
		{Name: "id", Type: "bigint", Unsigned: true, PrimaryKey: true, AutoIncrement: true, DesignType: "pk", Comment: "ID"},
		{Name: "title", Type: "varchar", Length: 100, Comment: "标题"},
		{Name: "admin_id", Type: "bigint", Comment: "归属"},
		{Name: "user_id", Type: "int", DesignType: "remoteSelect", Comment: "会员", Form: crudmodel.FormAttr{
			RemoteTable: "user", RemotePk: "id", RemoteField: "username", RelationFields: "username",
		}},
		{Name: "status", Type: "tinyint", Length: 1, Default: "1", DefaultType: "INPUT", DesignType: "switch", Comment: "状态:0=禁用,1=启用"},
		{Name: "create_time", Type: "bigint", Unsigned: true, Comment: "创建时间"},
		{Name: "update_time", Type: "bigint", Unsigned: true, Comment: "更新时间"},
	}
	content, err := MarshalSpec(table, fields)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ops_banner.yaml")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	options, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("load rewritten spec: %v\n%s", err, content)
	}
	if options.Table.Name != "ops_banner" || options.Table.Comment != "轮播图表" {
		t.Fatalf("table identity lost: %+v", options.Table)
	}
	if options.Type != "auto" {
		t.Fatalf("written spec must omit type so it derives, got %q", options.Type)
	}
	if len(options.Fields) != len(fields) {
		t.Fatalf("fields = %d, want %d", len(options.Fields), len(fields))
	}
	user := options.Fields[3]
	if user.Form.RemoteTable != "user" || user.Form.RemoteField != "username" || user.Form.RemotePk != "id" || user.Form.RelationFields != "username" {
		t.Fatalf("remote attributes lost: %+v", user.Form)
	}
	status := options.Fields[4]
	if status.DesignType != "switch" || status.Default != "1" || status.DefaultType != "INPUT" || status.Length != 1 {
		t.Fatalf("status field lost: %+v", status)
	}
	if len(options.Table.Indexes) != 1 || options.Table.Indexes[0].Name != "uk_title" || !options.Table.Indexes[0].Unique {
		t.Fatalf("indexes lost: %+v", options.Table.Indexes)
	}
	if options.Table.DataScope == nil || options.Table.DataScope.Mode != data_scope.ModeAuto {
		t.Fatalf("data scope lost: %+v", options.Table.DataScope)
	}
	if warnings, err := ValidateSpec(path); err != nil {
		t.Fatalf("rewritten spec does not validate: %v (%+v)", err, warnings)
	}
}

// 设计器入口把表设计写回 crud_specs；写出的文件必须能被 CLI 直接生成。
func TestWriteSpecOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	state := &generateState{opts: GenerateOptions{
		SpecOutputDir: dir,
		Table:         crudmodel.Table{Name: "ops_demo", Comment: "演示表"},
		Fields: []crudmodel.Field{
			{Name: "id", Type: "bigint", Unsigned: true, PrimaryKey: true, AutoIncrement: true, DesignType: "pk", Comment: "ID"},
			{Name: "name", Type: "varchar", Length: 50, Comment: "名称"},
		},
	}}
	state.prepareSpecOutput()
	if err := state.writeSpecOutput(); err != nil {
		t.Fatal(err)
	}
	options, err := LoadSpec(state.specPath)
	if err != nil {
		t.Fatalf("load written spec: %v", err)
	}
	if options.Table.Name != "ops_demo" || len(options.Fields) != 2 {
		t.Fatalf("round-trip mismatch: %+v", options.Table)
	}
}
