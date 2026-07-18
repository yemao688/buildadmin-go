package crud_helper

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSnapshotRestoresCreatedAndOverwrittenFiles(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.go")
	created := filepath.Join(dir, "created.go")
	if err := os.WriteFile(existing, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewFileSnapshot([]string{existing, created})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Cleanup()
	if err := os.WriteFile(existing, []byte("overwritten"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(created, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Restore(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(existing)
	if err != nil || string(data) != "original" {
		t.Fatalf("restored existing = %q, err=%v", data, err)
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatalf("created file still exists, err=%v", err)
	}
}

func TestQuarantineRestoresAllFiles(t *testing.T) {
	assertQuarantineRestore(t)
}

func TestBuildFileManifestForFieldsContainsExistingRelationProvider(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "app", "admin", "model", "relation_manifest_test")
	provider := filepath.Join(dir, "provider.go")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provider, []byte("package model\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	manifest, err := BuildFileManifestForFields(model.Table{Name: "orders"}, []model.Field{{Form: model.FormAttr{RemoteTable: "owner", RemoteModel: "app/admin/model/relation_manifest_test/Owner.go", RelationFields: "name"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range manifest.Shared {
		if path == provider {
			return
		}
	}
	t.Fatalf("existing relation provider %q was not snapshotted: %v", provider, manifest.Shared)
}

func assertQuarantineRestore(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp(utils.RootPath(), ".crud-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	first := filepath.Join(dir, "first.vue")
	second := filepath.Join(dir, "second.go")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte(path), 0644); err != nil {
			t.Fatal(err)
		}
	}
	quarantine, err := NewQuarantine([]string{first, second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("quarantined file remains at original path: %v", err)
	}
	if err := quarantine.Restore(); err != nil {
		t.Fatal(err)
	}
	if err := quarantine.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{first, second} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("restored file %s missing: %v", path, err)
		}
	}
}
