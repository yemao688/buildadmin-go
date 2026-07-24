package crud_helper

import (
	"go-build-admin/app/pkg/data_scope"
	"os"
	"path/filepath"
	"testing"
)

func writeSpecTest(t *testing.T, content string) string {
	t.Helper()
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
