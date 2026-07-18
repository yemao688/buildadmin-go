package crud_helper

import (
	"go-build-admin/app/admin/model"
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
