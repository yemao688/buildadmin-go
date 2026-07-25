package crud_helper

import (
	"fmt"
	"go-build-admin/app/pkg/data_scope"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSpecTest(t *testing.T, content string) string {
	t.Helper()
	content = strings.ReplaceAll(content, `\n`, "\n")
	path := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSpecDefaultsAndMenu(t *testing.T) {
	path := writeSpecTest(t, `name: demo
comment: Demo
menu:
  title: Demo menu
fields:
  - name: id
    type: bigint
    primaryKey: true
  - name: admin_id
    type: bigint
  - name: name
    type: varchar
    length: 32
`)
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Type != "create" || opts.Table.DataScope == nil || opts.Table.DataScope.Mode != data_scope.ModeAuto {
		t.Fatalf("defaults: type=%q scope=%v", opts.Type, opts.Table.DataScope)
	}
	if len(opts.Table.FormFields) != 2 || len(opts.Table.ColumnFields) != 3 {
		t.Fatalf("derived fields: form=%v columns=%v", opts.Table.FormFields, opts.Table.ColumnFields)
	}
	if opts.Menu == nil || opts.Menu.Title != "Demo menu" || opts.Menu.Parent != 0 {
		t.Fatalf("menu defaults: %+v", opts.Menu)
	}
	if opts.Fields[2].DesignType != "string" {
		t.Fatalf("design type = %q", opts.Fields[2].DesignType)
	}
}

func TestLoadSpecBindsLowercaseRemoteRelationKeys(t *testing.T) {
	path := writeSpecTest(t, `name: child
fields:
  - name: id
    type: bigint
    primaryKey: true
  - name: base_id
    type: bigint
    designType: remoteSelect
    form:
      remotetable: ai_gate_base
      remotepk: id
      remotefield: name
      remotemodel: ai_gate_base
      relationfields: name
`)
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	form := opts.Fields[1].Form
	if form.RemoteTable != "ai_gate_base" || form.RemotePk != "id" || form.RemoteField != "name" || form.RemoteModel != "ai_gate_base" || form.RelationFields != "name" {
		t.Fatalf("remote relation binding failed: %+v", form)
	}
}

func TestLoadSpecRejectsUnsafeField(t *testing.T) {
	path := writeSpecTest(t, `name: demo
fields:
  - name: "bad;drop"
    type: varchar
`)
	if _, err := LoadSpec(path); err == nil {
		t.Fatal("unsafe field was accepted")
	}
	path = writeSpecTest(t, `name: demo
fields:
  - name: name
    type: "varchar); drop table users;--"
`)
	if _, err := LoadSpec(path); err == nil {
		t.Fatal("unsafe type was accepted")
	}
}

func TestLoadSpecNormalizesNullKeys(t *testing.T) {
	path := writeSpecTest(t, `name: null_keys
fields:
  - name: id
    type: int
    primaryKey: true
  - name: unquoted_true
    type: varchar
    length: 8
    null: true
  - name: unquoted_false
    type: varchar
    length: 8
    null: false
  - name: omitted
    type: varchar
    length: 8
  - name: quoted_true
    type: varchar
    length: 8
    'null': true
  - name: tilde_false
    type: varchar
    length: 8
    ~: false
`)
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{false, true, false, false, true, false}
	if len(opts.Fields) != len(want) {
		t.Fatalf("field count = %d, want %d", len(opts.Fields), len(want))
	}
	for i, field := range opts.Fields {
		if field.Null != want[i] {
			t.Errorf("field %q Null=%v, want %v", field.Name, field.Null, want[i])
		}
	}
}

func TestLoadSpecTimestampDefaultsAndFormFieldDerivation(t *testing.T) {
	path := writeSpecTest(t, `name: timestamp_defaults
fields:
  - name: id
    type: bigint
    primaryKey: true
  - name: createTime
    type: datetime
  - name: update_time
    type: datetime
    formBuildExclude: false
    table:
      operator: EQUALS
      width: 240
  - name: amount
    type: bigint
  - name: internal_note
    type: varchar
    formBuildExclude: true
`)
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}

	if opts.Fields[1].DesignType != "timestamp" {
		t.Fatalf("createTime design type = %q, want timestamp", opts.Fields[1].DesignType)
	}
	defaults := opts.Fields[1].Table
	if defaults.Render != "datetime" || defaults.Operator != "RANGE" || defaults.ComSearchRender != "datetime" || defaults.Width != 160 || defaults.TimeFormat != "yyyy-mm-dd hh:MM:ss" {
		t.Fatalf("timestamp table defaults = %+v", defaults)
	}
	if opts.Fields[2].FormBuildExclude || opts.Fields[2].Table.Operator != "EQUALS" || opts.Fields[2].Table.Width != 240 {
		t.Fatalf("explicit timestamp overrides = %+v", opts.Fields[2])
	}
	if opts.Fields[3].DesignType != "number" {
		t.Fatalf("bigint design type = %q, want number", opts.Fields[3].DesignType)
	}
	if len(opts.Table.FormFields) != 2 || opts.Table.FormFields[0] != "update_time" || opts.Table.FormFields[1] != "amount" {
		t.Fatalf("derived form fields = %v", opts.Table.FormFields)
	}
}

func TestLoadSpecExplicitFormFieldsAreAuthoritative(t *testing.T) {
	path := writeSpecTest(t, `name: explicit_form_fields
formFields: []
fields:
  - name: id
    type: bigint
    primaryKey: true
  - name: createtime
    type: datetime
  - name: hidden
    type: varchar
    formBuildExclude: true
`)
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Table.FormFields == nil || len(opts.Table.FormFields) != 0 {
		t.Fatalf("explicit empty formFields were not preserved: %v", opts.Table.FormFields)
	}
}

func TestLoadSpecCompletenessOptions(t *testing.T) {
	tests := []struct {
		name, yaml, wantPath, wantDB string
		wantColumns                  int
	}{
		{"defaults", `name: orders\nfields:\n  - {name: id, type: bigint, primaryKey: true}\n  - {name: title, type: varchar}`, "", "mysql", 2},
		{"empty columns", `name: orders\ncolumnFields: []\nfields:\n  - {name: id, type: bigint, primaryKey: true}`, "", "mysql", 0},
		{"dotted shorthand", `name: country_language_content\ngenerateRelativePath: country.languageContent\nfields:\n  - {name: id, type: bigint, primaryKey: true}`, "country/languageContent", "mysql", 1},
		{"slash shorthand", `name: orders\ngenerateRelativePath: custom_dir/orders\ndatabaseConnection: mysql\nfields:\n  - {name: id, type: bigint, primaryKey: true}`, "custom_dir/orders", "mysql", 1},
		{"backslash shorthand", "name: orders\ngenerateRelativePath: custom_dir\\orders\nfields:\n  - {name: id, type: bigint, primaryKey: true}", "custom_dir/orders", "mysql", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := LoadSpec(writeSpecTest(t, tc.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if opts.Table.GenerateRelativePath != tc.wantPath || opts.Table.DatabaseConnection != tc.wantDB || len(opts.Table.ColumnFields) != tc.wantColumns {
				t.Fatalf("table=%+v", opts.Table)
			}
			if tc.wantPath != "" && (opts.Table.ModelFile != "app/admin/model/"+tc.wantPath+".go" || opts.Table.ControllerFile != "app/admin/handler/"+tc.wantPath+".go" || opts.Table.WebViewsDir != "web/src/views/backend/"+tc.wantPath) {
				t.Fatalf("derived paths: %+v", opts.Table)
			}
		})
	}
	for _, tc := range []struct{ name, yaml, model, handler, views string }{
		{"model override", `name: orders\ngenerateRelativePath: base/orders\nmodelFile: app/admin/model/custom_orders.go\nfields:\n  - {name: id, type: bigint, primaryKey: true}`, "app/admin/model/custom_orders.go", "app/admin/handler/base/orders.go", "web/src/views/backend/base/orders"},
		{"all overrides", `name: orders\ngenerateRelativePath: base/orders\nmodelFile: app/admin/model/custom.go\ncontrollerFile: app/admin/handler/custom.go\nwebViewsDir: web/src/views/backend/custom\nfields:\n  - {name: id, type: bigint, primaryKey: true}`, "app/admin/model/custom.go", "app/admin/handler/custom.go", "web/src/views/backend/custom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := LoadSpec(writeSpecTest(t, tc.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if opts.Table.ModelFile != tc.model || opts.Table.ControllerFile != tc.handler || opts.Table.WebViewsDir != tc.views {
				t.Fatalf("overrides lost: %+v", opts.Table)
			}
		})
	}
}

func TestLoadSpecDefaultTypesAndRemoteAlias(t *testing.T) {
	yaml := `name: defaults
fields:
  - {name: id, type: bigint, primaryKey: true}
  - {name: none_value, type: varchar, defaultType: none}
  - {name: null_value, type: varchar, defaultType: NULL}
  - {name: empty_value, type: varchar, defaultType: empty string}
  - {name: input_value, type: varchar, defaultType: input, default: ""}
  - name: owner_id
    type: bigint
    designType: remoteSelect
    table:
      comSearchInputAttr: {size: large}
    form:
      remotePrimaryTableAlias: owner
      remotePk: uuid
`
	opts, err := LoadSpec(writeSpecTest(t, yaml))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"NONE", "NULL", "EMPTY STRING", "INPUT"}
	for i, value := range want {
		if opts.Fields[i+1].DefaultType != value {
			t.Errorf("default type %d=%q", i, opts.Fields[i+1].DefaultType)
		}
	}
	if opts.Fields[5].Table.ComSearchInputAttr["size"] != "large" || opts.Fields[5].Form.RemotePrimaryTableAlias != "owner" {
		t.Fatalf("field config: %+v", opts.Fields[5])
	}
}

func TestLoadSpecSearchInputAttrTextIsTypedAndDeterministic(t *testing.T) {
	path := writeSpecTest(t, "name: attrs\nfields:\n  - {name: id, type: bigint, primaryKey: true}\n  - name: title\n    type: varchar\n    table:\n      comSearchInputAttr: |\n        z-index=2\n        disabled=false\n        \n        placeholder=a=b\n")
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	attrs := opts.Fields[1].Table.ComSearchInputAttr
	if attrs["disabled"] != false || attrs["z-index"] != 2 || attrs["placeholder"] != "a=b" {
		t.Fatalf("attrs = %#v", attrs)
	}
	got := getJsonFromAny(attrs)
	if got != `{ disabled: false, placeholder: "a=b", "z-index": 2 }` {
		t.Fatalf("serialized attrs = %q", got)
	}
}

func TestLoadSpecRejectsInvalidSearchInputAttr(t *testing.T) {
	path := writeSpecTest(t, "name: attrs\nfields:\n  - {name: id, type: bigint, primaryKey: true}\n  - name: title\n    type: varchar\n    table:\n      comSearchInputAttr: |\n        invalid-entry\n")
	if _, err := LoadSpec(path); err == nil {
		t.Fatal("invalid search input attr accepted")
	}
}

func TestLoadSpecRejectsInvalidCompletenessOptions(t *testing.T) {
	for _, yaml := range []string{
		`name: bad\ndatabaseConnection: postgres\nfields:\n  - {name: id, type: int, primaryKey: true}`,
		`name: bad\ngenerateRelativePath: ../escape\nfields:\n  - {name: id, type: int, primaryKey: true}`,
		`name: bad\nfields:\n  - {name: id, type: int, primaryKey: true}\n  - {name: x, type: varchar, defaultType: invalid}`,
		"name: bad\nfields:\n  - {name: id, type: int, primaryKey: true}\n  - name: x\n    type: varchar\n    table:\n      comSearchInputAttr: |\n        .a=bad\n",
	} {
		if _, err := LoadSpec(writeSpecTest(t, yaml)); err == nil {
			t.Errorf("invalid spec accepted: %s", yaml)
		}
	}
}

func TestLoadSpecRemotePkValidation(t *testing.T) {
	base := "name: orders\nfields:\n  - {name: id, type: int, primaryKey: true}\n  - name: owner_id\n    type: int\n    form:\n      remoteTable: owner\n      remotePk: %s\n"
	for _, remotePk := range []string{"uuid", "owner.uuid"} {
		if _, err := LoadSpec(writeSpecTest(t, fmt.Sprintf(base, remotePk))); err != nil {
			t.Errorf("remotePk %q rejected: %v", remotePk, err)
		}
	}
	for _, remotePk := range []string{".uuid", "uuid.", "owner..uuid", "a.b.c"} {
		if _, err := LoadSpec(writeSpecTest(t, fmt.Sprintf(base, remotePk))); err == nil {
			t.Errorf("malformed remotePk %q was accepted", remotePk)
		}
	}
}

func TestLoadSpecBooleanMultiFlagsPreserveFalseAndTrue(t *testing.T) {
	path := writeSpecTest(t, `name: boolean_flags
fields:
  - {name: id, type: int, primaryKey: true}
  - name: owner_id
    type: int
    designType: remoteSelect
    form:
      selectMulti: false
      remoteTable: owner
  - name: owner_ids
    type: varchar
    designType: remoteSelect
    form:
      selectMulti: true
      remoteTable: owner
  - name: cover_image
    type: varchar
    designType: image
    form:
      imageMulti: false
  - name: cover_images
    type: varchar
    designType: image
    form:
      imageMulti: true
  - name: document_file
    type: varchar
    designType: file
    form:
      fileMulti: false
  - name: document_files
    type: varchar
    designType: file
    form:
      fileMulti: true
`)
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Fields[1].Form.SelectMulti != "" || analyseField(opts.Fields[1]).DesignType != "remoteSelect" {
		t.Fatalf("false selectMulti promoted field: %+v", opts.Fields[1])
	}
	if opts.Fields[2].Form.SelectMulti != "1" || analyseField(opts.Fields[2]).DesignType != "remoteSelects" {
		t.Fatalf("true selectMulti did not promote field: %+v", opts.Fields[2])
	}
	if opts.Fields[3].Form.ImageMulti != "" || analyseField(opts.Fields[3]).DesignType != "image" || opts.Fields[5].Form.FileMulti != "" || analyseField(opts.Fields[5]).DesignType != "file" {
		t.Fatalf("false image/file flags changed design types: %+v %+v", opts.Fields[3], opts.Fields[5])
	}
	if opts.Fields[4].Form.ImageMulti != "1" || analyseField(opts.Fields[4]).DesignType != "images" || opts.Fields[6].Form.FileMulti != "1" || analyseField(opts.Fields[6]).DesignType != "files" {
		t.Fatalf("true image/file flags did not promote design types: %+v %+v", opts.Fields[4], opts.Fields[6])
	}
	overrides := buildModelFieldTypeOverrides(opts.Fields)
	if _, ok := overrides["owner_id"]; ok {
		t.Fatalf("false select flag created adapter override: %+v", overrides)
	}
	if overrides["owner_ids"] != "validate.CommaJoined" || overrides["cover_images"] != "validate.CommaJoined" || overrides["document_files"] != "validate.CommaJoined" {
		t.Fatalf("true multi flags missing adapter overrides: %+v", overrides)
	}
}

func TestLoadSpecPreservesFractionalNumberStep(t *testing.T) {
	path := writeSpecTest(t, `name: fractional_step
fields:
  - {name: id, type: int, primaryKey: true}
  - name: ratio
    type: decimal
    designType: number
    form:
      step: 0.001
`)
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Fields[1].Form.Step != 0.001 {
		t.Fatalf("fractional step = %v", opts.Fields[1].Form.Step)
	}
}

func TestLoadSpecNullDefaultNormalizesNullability(t *testing.T) {
	path := writeSpecTest(t, "name: null_defaults\nfields:\n  - {name: id, type: int, primaryKey: true}\n  - {name: nullable, type: varchar, defaultType: NULL}\n  - {name: explicit, type: varchar, defaultType: NULL, null: false}\n")
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	if !opts.Fields[1].Null || !opts.Fields[2].Null {
		t.Fatalf("NULL defaults were not normalized: %+v", opts.Fields[1:])
	}
}

func TestLoadSpecDefaultsDoNotOverrideExplicitAttributes(t *testing.T) {
	path := writeSpecTest(t, `name: explicit_attrs
fields:
  - name: id
    type: bigint
    primaryKey: true
    autoIncrement: true
  - name: amount_number
    type: int
    table:
      render: custom
      operator: EQUALS
      sortable: custom
      width: 240
    form:
      step: 5
      validator: [required]
  - name: owner_id
    type: bigint
    designType: remoteSelect
    form:
      remotePk: uuid
      remoteField: nickname
`)
	opts, err := LoadSpec(path)
	if err != nil {
		t.Fatal(err)
	}
	amount := opts.Fields[1]
	if amount.Table.Render != "custom" || amount.Table.Operator != "EQUALS" || amount.Table.Sortable != "custom" || amount.Table.Width != 240 || amount.Form.Step != 5 || len(amount.Form.Validator) != 1 || amount.Form.Validator[0] != "required" {
		t.Fatalf("explicit defaults were overwritten: %+v", amount)
	}
	remote := opts.Fields[2].Form
	if remote.RemotePk != "uuid" || remote.RemoteField != "nickname" {
		t.Fatalf("explicit remote attrs were overwritten: %+v", remote)
	}
}
