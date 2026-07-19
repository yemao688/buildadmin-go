package crud_helper

import (
	"go-build-admin/app/admin/model"
	"strconv"
	"strings"
	"testing"
)

func TestValidateGenerationInputRejectsInjectedIdentifiersAndTypes(t *testing.T) {
	base := model.Table{Name: "orders"}
	if err := ValidateGenerationInput(base, []model.Field{{Name: "name`); DROP TABLE users;--", Type: "varchar"}}); err == nil {
		t.Fatal("injected field name was accepted")
	}
	if err := ValidateGenerationInput(base, []model.Field{{Name: "name", DataType: "varchar(20)); DROP TABLE users;--"}}); err == nil {
		t.Fatal("injected data type was accepted")
	}
	if err := ValidateGenerationInput(model.Table{Name: "orders; DROP TABLE users"}, nil); err == nil {
		t.Fatal("injected table name was accepted")
	}
}

func TestValidateGenerationInputAllowsDesignerTypesAndEscapesSQLStrings(t *testing.T) {
	field := model.Field{Name: "title", DataType: "varchar(64)", Default: "O'Reilly", Comment: "Bob's title", PrimaryKey: true}
	if err := ValidateGenerationInput(model.Table{Name: "orders", Comment: "customer's orders"}, []model.Field{field}); err != nil {
		t.Fatal(err)
	}
	if got := formatDefault(field.Default); got != "DEFAULT 'O''Reilly'" {
		t.Fatalf("default escaping = %q", got)
	}
	if got := formatComment(field.Comment); got != "COMMENT 'Bob''s title'" {
		t.Fatalf("comment escaping = %q", got)
	}
}

func TestValidateGenerationInputRequiresOneUniquePrimaryKey(t *testing.T) {
	base := model.Table{Name: "orders"}
	if err := ValidateGenerationInput(base, []model.Field{{Name: "id", Type: "int"}}); err == nil {
		t.Fatal("missing primary key was accepted")
	}
	if err := ValidateGenerationInput(base, []model.Field{{Name: "id", Type: "int", PrimaryKey: true}, {Name: "ID", Type: "varchar"}}); err == nil {
		t.Fatal("case-insensitive duplicate field was accepted")
	}
	if err := ValidateGenerationInput(base, []model.Field{{Name: "id", Type: "int", PrimaryKey: true}, {Name: "other", Type: "int", PrimaryKey: true}}); err == nil {
		t.Fatal("multiple primary keys were accepted")
	}
}

func TestValidateGenerationInputRejectsUnsafeRemoteModel(t *testing.T) {
	field := model.Field{
		Name: "owner_id", Type: "int", Form: model.FormAttr{
			RemoteTable: "owner", RemoteModel: "app/admin/model/../handler/Evil.go", RelationFields: "name",
		},
	}
	err := ValidateGenerationInput(model.Table{Name: "orders"}, []model.Field{{Name: "id", Type: "int", PrimaryKey: true}, field})
	if err == nil {
		t.Fatal("remote model path traversal was accepted")
	}
}

func TestValidatePathUnderRootsRejectsTraversalAndAbsolutePaths(t *testing.T) {
	for _, path := range []string{"../outside.go", "/tmp/outside.go", `..\\outside.go`} {
		if err := ValidatePathUnderRoots(path, "app/admin/model"); err == nil {
			t.Errorf("path %q was accepted", path)
		}
	}
	if err := ValidateGeneratedAbsolutePath("/tmp/outside.go", "app/admin/model"); err == nil {
		t.Fatal("absolute path outside root was accepted")
	}
}

func TestValidateGenerationInputRejectsUnsafeFrontendValues(t *testing.T) {
	base := model.Table{Name: "orders"}
	for _, field := range []model.Field{
		{Name: "owner", Type: "int", Form: model.FormAttr{RemoteField: "name;alert(1)"}},
		{Name: "owner", Type: "int", Form: model.FormAttr{RemoteController: "app/admin/controller/../Evil.go"}},
		{Name: "owner", Type: "int", Form: model.FormAttr{RemoteUrl: "javascript:alert(1)"}},
		{Name: "owner", Type: "int", Form: model.FormAttr{RemoteUrl: "data:text/html,x"}},
		{Name: "owner", Type: "int", Form: model.FormAttr{RemoteUrl: "/api\nEvil"}},
		{Name: "owner", Type: "int", Form: model.FormAttr{RemoteUrl: "/api/'\""}},
	} {
		if err := ValidateGenerationInput(base, []model.Field{{Name: "id", Type: "int", PrimaryKey: true}, field}); err == nil {
			t.Errorf("unsafe frontend field was accepted: %+v", field.Form)
		}
	}
}

func TestRenderFormFileUsesSafeJavaScriptStrings(t *testing.T) {
	fields := []model.Field{{Name: "name", Form: model.FormAttr{Validator: []string{"required"}, ValidatorMsg: "bad ' quote\nnext"}}}
	content, err := renderFormFile(FormVueData{}, fields, "form.")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(content, "message: 'bad '") || strings.Contains(content, "message: 'bad ' quote") {
		t.Fatalf("validator message was emitted as an unsafe single-quoted string: %s", content)
	}
	if !strings.Contains(content, `message: "bad ' quote\nnext"`) {
		t.Fatalf("validator message was not JSON/JS encoded: %s", content)
	}
}

func TestBuildTableColumnKeyOnlyTrustsCompleteTranslationCalls(t *testing.T) {
	legitimate := buildTableColumnKey("label", `t("orders.name")`)
	if !strings.Contains(legitimate, `label: t("orders.name")`) {
		t.Fatalf("legitimate translation call was quoted: %s", legitimate)
	}

	malicious := `t('orders.name');globalThis.pwned=true//`
	encoded := buildTableColumnKey("label", malicious)
	if strings.Contains(encoded, `label: t('orders.name');`) {
		t.Fatalf("translation prefix escaped the string literal: %s", encoded)
	}
	if !strings.Contains(encoded, strconv.Quote(malicious)) {
		t.Fatalf("untrusted label was not encoded: %s", encoded)
	}
}
