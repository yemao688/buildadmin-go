package crudgen

import (
	entity "buildadmin-go/internal/model"
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/util"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSnapshotRestoresCreatedAndOverwrittenFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.go")
	created := filepath.Join(dir, "created.go")
	if err := os.WriteFile(existing, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewFileSnapshot([]string{existing, created})
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Cleanup()
	if err := os.WriteFile(existing, []byte("overwritten"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(created, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Restore(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(existing) //nolint:gosec // G304: existing 为测试内 t.TempDir 夹具路径
	if err != nil || string(data) != "original" {
		t.Fatalf("restored existing = %q, err=%v", data, err)
	}
	if _, err := os.Stat(created); !os.IsNotExist(err) {
		t.Fatalf("created file still exists, err=%v", err)
	}
}

func TestQuarantineRestoresAllFiles(t *testing.T) {
	t.Parallel()
	assertQuarantineRestore(t)
}

func TestBuildFileManifestForFieldsContainsExistingRelationProvider(t *testing.T) {
	t.Parallel()
	// 拍平布局：关联仓库的 provider 即扁平根包 provider.go（wire 静态聚合）。
	provider := filepath.Join(util.RootPath(), "internal", "admin", "repository", "provider.go")
	manifest, err := BuildFileManifestForFields(crudmodel.Table{Name: "orders"}, []crudmodel.Field{{Form: crudmodel.FormAttr{RemoteTable: "owner", RemoteModel: "internal/model/owner.go", RelationFields: "name"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range manifest.Shared {
		if path == provider {
			return
		}
	}
	t.Fatalf("relation provider %q was not snapshotted: %v", provider, manifest.Shared)
}

func TestBuildFileManifestForFieldsAlwaysClassifiesRelationProviderAsShared(t *testing.T) {
	t.Parallel()
	manifest, err := BuildFileManifestForFields(crudmodel.Table{Name: "orders"}, []crudmodel.Field{{Form: crudmodel.FormAttr{RemoteTable: "owner", RemoteModel: "internal/model/owner.go", RelationFields: "name"}}})
	if err != nil {
		t.Fatal(err)
	}
	provider := filepath.Join(util.RootPath(), "internal", "admin", "repository", "provider.go")
	if containsPath(manifest.Generated, provider) || !containsPath(manifest.Shared, provider) {
		t.Fatalf("relation provider classification = %+v", manifest)
	}
	// 实体尚未存在时应进入 Generated（新布局共享记录层，扁平 internal/model/<table>.go）
	entity := filepath.Join(util.RootPath(), "internal", "model", "owner.go")
	if !containsPath(manifest.Generated, entity) {
		t.Fatalf("relation entity missing from generated manifest: %+v", manifest.Generated)
	}
}

// crud_log 清单必须存相对仓库根路径：同一份清单在另一个 checkout 写入后，
// 当前检出重新生成时不能被旧根目录的绝对前缀拒绝。
func TestPortableTableForLogStoresRootRelativePaths(t *testing.T) {
	t.Parallel()
	root := util.RootPath()
	table := crudmodel.Table{
		GeneratedFiles: []string{
			filepath.Join(root, "internal", "admin", "handler", "goods_product.go"),
			filepath.Join(root, "web", "src", "views", "backend", "goodsProduct", "index.vue"),
		},
		Manifest: &crudmodel.CRUDFileManifest{
			Generated: []string{filepath.Join(root, "internal", "model", "goods_product.go")},
			Shared:    []string{filepath.Join(root, "internal", "admin", "repository", "provider.go")},
		},
	}
	portable := portableTableForLog(table)
	wantGenerated := []string{"internal/admin/handler/goods_product.go", "web/src/views/backend/goodsProduct/index.vue"}
	for i, want := range wantGenerated {
		if portable.GeneratedFiles[i] != want {
			t.Fatalf("GeneratedFiles[%d] = %q, want %q", i, portable.GeneratedFiles[i], want)
		}
	}
	if got := portable.Manifest.Generated[0]; got != "internal/model/goods_product.go" {
		t.Fatalf("manifest generated = %q", got)
	}
	if got := portable.Manifest.Shared[0]; got != "internal/admin/repository/provider.go" {
		t.Fatalf("manifest shared = %q", got)
	}
	if !filepath.IsAbs(table.GeneratedFiles[0]) {
		t.Fatal("portableTableForLog must not mutate the in-memory table used for file operations")
	}
}

// 两个不同 checkout 根目录记录同一份清单时，manifestAllows 必须视为等价；
// 真正的路径漂移仍然拒绝。
func TestManifestAllowsAcrossCheckoutRoots(t *testing.T) {
	t.Parallel()
	rootA := filepath.Join(t.TempDir(), "checkout-a")
	rootB := filepath.Join(t.TempDir(), "checkout-b")
	paths := func(root string) []string {
		return []string{
			filepath.Join(root, "internal", "model", "goods_product.go"),
			filepath.Join(root, "internal", "admin", "handler", "goods_product.go"),
			filepath.Join(root, "internal", "admin", "repository", "provider.go"),
			filepath.Join(root, "cmd", "server", "wire_gen.go"),
		}
	}

	manifest := FileManifest{Generated: paths(rootB)[:2], Shared: paths(rootB)[2:]}
	log := &entity.Log{Table: crudmodel.JSON_TABLE{GeneratedFiles: paths(rootA)}}
	if !manifestAllowsAt(manifest, log, nil, rootB) {
		t.Fatal("manifest recorded in another checkout root must still be accepted")
	}

	// 新写入的记录是相对路径，同样必须与当前根的绝对清单匹配。
	relativeLog := &entity.Log{Table: crudmodel.JSON_TABLE{GeneratedFiles: []string{
		"internal/model/goods_product.go",
		"internal/admin/handler/goods_product.go",
		"internal/admin/repository/provider.go",
		"cmd/server/wire_gen.go",
	}}}
	if !manifestAllowsAt(manifest, relativeLog, nil, rootB) {
		t.Fatal("relative manifest paths must resolve against the current root")
	}

	drifted := FileManifest{Generated: []string{
		filepath.Join(rootB, "internal", "model", "other.go"),
		filepath.Join(rootB, "internal", "admin", "handler", "goods_product.go"),
	}, Shared: paths(rootB)[2:]}
	if manifestAllowsAt(drifted, log, nil, rootB) {
		t.Fatal("path drift must still be rejected")
	}
}

func TestManifestPathForRootRebasesLegacyAbsolutePaths(t *testing.T) {
	t.Parallel()
	current := filepath.Join(t.TempDir(), "current")
	legacy := filepath.Join(t.TempDir(), "legacy")
	nestedRoot := filepath.Join(t.TempDir(), "internal", "model", "checkout")
	cases := map[string]string{
		filepath.Join(legacy, "internal", "admin", "repository", "provider.go"):           filepath.Join(current, "internal", "admin", "repository", "provider.go"),
		filepath.Join(legacy, "cmd", "server", "wire_gen.go"):                             filepath.Join(current, "cmd", "server", "wire_gen.go"),
		filepath.Join(legacy, "web", "src", "lang", "backend", "en", "x.ts"):              filepath.Join(current, "web", "src", "lang", "backend", "en", "x.ts"),
		"internal/admin/handler/goods.go":                                                 filepath.Join(current, "internal", "admin", "handler", "goods.go"),
		filepath.Join(nestedRoot, "internal", "admin", "handler", "goods.go"):             filepath.Join(current, "internal", "admin", "handler", "goods.go"),
		filepath.Join(nestedRoot, "web", "src", "views", "backend", "goods", "index.vue"): filepath.Join(current, "web", "src", "views", "backend", "goods", "index.vue"),
	}
	for input, want := range cases {
		if got := manifestPathForRoot(input, current); got != want {
			t.Fatalf("manifestPathForRoot(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFileSnapshotReadFailureLeavesTargetsUntouched(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "existing.go")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
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
	if err := os.WriteFile(path, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := snapshot.Restore(); err == nil {
		t.Fatal("missing backup should fail")
	}
	data, err := os.ReadFile(path) //nolint:gosec // G304: path 为测试内 t.TempDir 夹具路径
	if err != nil || string(data) != "changed" {
		t.Fatalf("target changed after backup read failure: %q, %v", data, err)
	}
}

func assertQuarantineRestore(t *testing.T) {
	t.Helper()
	dir, err := os.MkdirTemp(util.RootPath(), ".crud-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	first := filepath.Join(dir, "first.vue")
	second := filepath.Join(dir, "second.go")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte(path), 0o600); err != nil {
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
