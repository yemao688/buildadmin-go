package crud_helper

import (
	crudmodel "buildadmin-go/internal/admin/model/crud"
	"buildadmin-go/internal/utils"
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
	dir := filepath.Join(utils.RootPath(), "internal", "admin", "repository", "relation_manifest_test")
	provider := filepath.Join(dir, "provider.go")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provider, []byte("package relation_manifest_test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	manifest, err := BuildFileManifestForFields(crudmodel.Table{Name: "orders"}, []crudmodel.Field{{Form: crudmodel.FormAttr{RemoteTable: "owner", RemoteModel: "internal/admin/repository/relation_manifest_test/Owner.go", RelationFields: "name"}}})
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

func TestBuildFileManifestForFieldsAlwaysClassifiesRelationProviderAsShared(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "internal", "admin", "repository", "relation_manifest_absent_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	manifest, err := BuildFileManifestForFields(crudmodel.Table{Name: "orders"}, []crudmodel.Field{{Form: crudmodel.FormAttr{RemoteTable: "owner", RemoteModel: "internal/admin/repository/relation_manifest_absent_test/Owner.go", RelationFields: "name"}}})
	if err != nil {
		t.Fatal(err)
	}
	provider := filepath.Join(dir, "provider.go")
	if containsPath(manifest.Generated, provider) || !containsPath(manifest.Shared, provider) {
		t.Fatalf("relation provider classification = %+v", manifest)
	}
	// 实体尚未存在时应进入 Generated（新布局共享记录层，扁平 internal/model）
	entity := filepath.Join(utils.RootPath(), "internal", "model", "Owner.go")
	if !containsPath(manifest.Generated, entity) {
		t.Fatalf("relation entity missing from generated manifest: %+v", manifest.Generated)
	}
}

func TestFileSnapshotReadFailureLeavesTargetsUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.go")
	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewFileSnapshot([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Cleanup()
	if err := os.Remove(snapshot.entries[0].backup); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Restore(); err == nil {
		t.Fatal("missing backup should fail")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "changed" {
		t.Fatalf("target changed after backup read failure: %q, %v", data, err)
	}
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
