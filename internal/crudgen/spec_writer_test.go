package crudgen

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/data_scope"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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
		Hooks:                &crudmodel.Hooks{AfterMutation: "currency-cache"},
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
	options, content, path := rewriteSpecThroughDisk(t, table, fields)
	assertRewrittenSpec(t, options, fields, path)
	if !strings.Contains(string(content), "remoteTable: user") {
		t.Fatalf("written spec must use camelCase attribute keys:\n%s", content)
	}
}

// rewriteSpecThroughDisk 把设计写回临时 spec 再读回来，返回读回结果、原始内容与路径。
func rewriteSpecThroughDisk(t *testing.T, table crudmodel.Table, fields []crudmodel.Field) (*GenerateOptions, []byte, string) {
	t.Helper()
	content, err := MarshalSpec(table, fields)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ops_banner.yaml")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	options, err := LoadSpec(path)
	if err != nil {
		t.Fatalf("load rewritten spec: %v\n%s", err, content)
	}
	return options, content, path
}

func assertRewrittenSpec(t *testing.T, options *GenerateOptions, fields []crudmodel.Field, path string) {
	t.Helper()
	assertRewrittenIdentity(t, options, fields)
	assertRewrittenFieldAttrs(t, options)
	if warnings, err := ValidateSpec(path); err != nil {
		t.Fatalf("rewritten spec does not validate: %v (%+v)", err, warnings)
	}
}

func assertRewrittenIdentity(t *testing.T, options *GenerateOptions, fields []crudmodel.Field) {
	t.Helper()
	if options.Table.Name != "ops_banner" || options.Table.Comment != "轮播图表" {
		t.Fatalf("table identity lost: %+v", options.Table)
	}
	if options.Type != "auto" {
		t.Fatalf("written spec must omit type so it derives, got %q", options.Type)
	}
	if len(options.Fields) != len(fields) {
		t.Fatalf("fields = %d, want %d", len(options.Fields), len(fields))
	}
	if options.Table.DataScope == nil || options.Table.DataScope.Mode != data_scope.ModeAuto {
		t.Fatalf("data scope lost: %+v", options.Table.DataScope)
	}
	if options.Table.Hooks == nil || options.Table.Hooks.AfterMutation != "currency-cache" {
		t.Fatalf("hooks lost: %+v", options.Table.Hooks)
	}
}

func assertRewrittenFieldAttrs(t *testing.T, options *GenerateOptions) {
	t.Helper()
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

// 设计器只建模部分键：改写既有 spec 时 menu/indexes 与 dataScope 的归属声明
// 必须保留，type 必须丢弃（省略即 auto）。
func TestMergeSpecPreservesDesignerUnmodeledKeys(t *testing.T) {
	t.Parallel()
	existing := []byte(`name: seller_user
type: create
menu:
  title: 卖家
  weigh: 3
indexes:
  - name: uk_existing
    unique: true
    columns: [name]
dataScope:
  mode: auto
  ownerColumn: admin_id
  assignOnCreate: false
  reassignable: true
  inheritFrom: {table: user, byColumn: user_id}
fields:
  - {name: id, type: bigint}
`)
	assignOnCreate := false
	table := crudmodel.Table{
		Name: "seller_user",
		DataScope: &data_scope.Config{
			Mode: data_scope.ModeAuto, OwnerColumn: "admin_id", AssignOnCreate: &assignOnCreate,
		},
		Indexes: []crudmodel.IndexSpec{{Name: "uk_new", Unique: true, Columns: []string{"name"}}},
	}
	generated, err := MarshalSpec(table, []crudmodel.Field{{Name: "id", Type: "bigint"}})
	require.NoError(t, err)
	merged, err := MergeSpec(existing, generated)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(merged, &doc))
	_, hasType := doc["type"]
	require.False(t, hasType, "type must be dropped so the next run derives it from the database")
	require.Contains(t, doc, "menu")
	require.Contains(t, doc, "indexes")
	scope, ok := doc["dataScope"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, scope["reassignable"])
	require.Equal(t, map[string]any{"table": "user", "byColumn": "user_id"}, scope["inheritFrom"])
	require.Equal(t, false, scope["assignOnCreate"])
}

// MarshalSpec 写出的 dataScope/indexes 必须能被 owner 登记（yaml 小写键）识别。
func TestMarshalSpecOwnerKeysAreReadable(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content, err := MarshalSpec(crudmodel.Table{
		Name: "seller_user",
		DataScope: &data_scope.Config{
			Mode:         data_scope.ModeAuto,
			OwnerColumn:  "admin_id",
			Reassignable: true,
			InheritFrom:  &data_scope.InheritRef{Table: "user", ByColumn: "user_id"},
		},
	}, []crudmodel.Field{{Name: "id", Type: "bigint"}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "seller_user.yaml"), content, 0o600))
	tables, err := CollectOwnerTables(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"seller_user"}, tables)
}

// 保留的 indexes 可能与新字段集不兼容（如字段改宽后需要前缀）：合并结果必须
// 先校验，失败时降级为丢弃保留项，否则会写出让 serve/migrate 直接失败的 spec。
func TestMergeSpecFallbackDropsIncompatibleIndexes(t *testing.T) {
	t.Parallel()
	existing := []byte(`name: ops_index_probe
fields:
  - {name: id, type: bigint, unsigned: true, primaryKey: true, autoIncrement: true}
  - {name: note, type: varchar, length: 100}
indexes:
  - name: uk_note
    unique: true
    columns: [note]
`)
	generated := []byte(`name: ops_index_probe
fields:
  - {name: id, type: bigint, unsigned: true, primaryKey: true, autoIncrement: true}
  - {name: note, type: longtext}
`)
	merged, err := MergeSpec(existing, generated)
	require.NoError(t, err)
	require.Error(t, ValidateSpecContent(merged), "longtext index without prefix must be rejected")

	fallback, err := mergeSpec(existing, generated, false)
	require.NoError(t, err)
	require.NoError(t, ValidateSpecContent(fallback), "fallback must drop the incompatible index")
}
