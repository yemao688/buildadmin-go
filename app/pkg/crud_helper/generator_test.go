package crud_helper

import (
	"errors"
	"go-build-admin/app/admin/model"
	"os"
	"testing"
)

func TestGenerateFromSpecRejectsProtectedTableBeforeDependencies(t *testing.T) {
	_, err := GenerateFromSpec(nil, nil, GenerateOptions{Table: model.Table{Name: "ba_admin"}})
	if err == nil || err.Error() != `crud generation is forbidden for protected table "ba_admin"` {
		t.Fatalf("error = %v", err)
	}
}

func TestDeleteQuarantinePathIsReusableByService(t *testing.T) {
	// This exercises the same quarantine primitive used by DeleteFromSpec: a
	// failure before commit restores every manifest member.
	assertQuarantineRestore(t)
}

func TestCreateExistingTableRequiresExplicitRebuild(t *testing.T) {
	if err := validateGenerationMode("create", "", true, "orders"); err == nil {
		t.Fatal("create on an existing table must be rejected")
	}
	if err := validateGenerationMode("create", "Yes", true, "orders"); err != nil {
		t.Fatal(err)
	}
	if err := validateGenerationMode("alter", "", true, "orders"); err != nil {
		t.Fatal(err)
	}
}

func TestAlterChangesOnlyAddAndModify(t *testing.T) {
	changes := deriveAlterChanges([]model.Column{{COLUMN_NAME: "id"}, {COLUMN_NAME: "legacy"}}, []model.Field{{Name: "id"}, {Name: "name"}})
	if len(changes) != 2 || changes[0].Type != "change-field-attr" || changes[1].Type != "add-field" {
		t.Fatalf("unexpected alter changes: %+v", changes)
	}
}

func TestManifestAllowsOnlyLatestSuccessfulTargets(t *testing.T) {
	path := t.TempDir() + "/model.go"
	manifest := FileManifest{Generated: []string{path}}
	if !manifestAllows(manifest, nil) {
		t.Fatal("first generation without a success manifest should be allowed")
	}
	if err := os.WriteFile(path, []byte("package model"), 0644); err != nil {
		t.Fatal(err)
	}
	if manifestAllows(manifest, nil) {
		t.Fatal("first generation must reject an existing target")
	}
	handlerPath := path + ".handler"
	if err := os.WriteFile(handlerPath, []byte("package handler"), 0644); err != nil {
		t.Fatal(err)
	}
	if manifestAllows(FileManifest{Generated: []string{path, handlerPath}}, nil) {
		t.Fatal("first generation must reject existing model and handler targets")
	}
	log := &model.CrudLog{Table: model.JSON_TABLE{GeneratedFiles: []string{path}}}
	if !manifestAllows(manifest, log) {
		t.Fatal("latest success manifest should allow its own target")
	}
	if manifestAllows(FileManifest{Generated: []string{path, path + ".new"}}, log) {
		t.Fatal("manifest path migration should be rejected")
	}
}

func TestCompileFailureRestoresSnapshot(t *testing.T) {
	path := t.TempDir() + "/generated.go"
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewFileSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Cleanup()
	if err := os.WriteFile(path, []byte("broken"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := buildAndRestoreOnFailure(snapshot, func() error { return errors.New("compile failed") }); err == nil {
		t.Fatal("compile failure should be returned")
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "original" {
		t.Fatalf("snapshot was not restored: %q, %v", content, err)
	}
}

func TestGenerationPanicErrorIsReadable(t *testing.T) {
	if got := generationPanicError("migrator panic").Error(); got != "panic: migrator panic" {
		t.Fatalf("panic error = %q", got)
	}
}

func TestFailedGenerationUnregistersRegisteredRoutes(t *testing.T) {
	routes := []atomicRouteRegistration{{method: "POST", path: "demo/add"}, {method: "DELETE", path: "demo/del"}}
	var got []atomicRouteRegistration
	unregisterAtomicRoutes(func(method, path string) { got = append(got, atomicRouteRegistration{method: method, path: path}) }, routes)
	if len(got) != 2 || got[0].path != "demo/del" || got[1].path != "demo/add" {
		t.Fatalf("unexpected unregister order: %+v", got)
	}
}
