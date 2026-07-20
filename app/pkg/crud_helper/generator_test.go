package crud_helper

import (
	"errors"
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"strings"
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

func TestBuildFileManifestUsesLocaleFirstLanguagePaths(t *testing.T) {
	tests := []struct {
		name         string
		viewsPath    string
		enLanguage   string
		zhCNLanguage string
	}{
		{"country_language_content", "web/src/views/backend/country/language/content", "country/language/content.ts", "country/language/content.ts"},
		{"test1", "web/src/views/backend/test1", "test1.ts", "test1.ts"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest, err := BuildFileManifest(model.Table{
				Name:           tc.name,
				ModelFile:      "app/admin/model/" + tc.name + ".go",
				ControllerFile: "app/admin/handler/" + tc.name + ".go",
				WebViewsDir:    tc.viewsPath,
			})
			if err != nil {
				t.Fatal(err)
			}
			expectedBase := filepath.Join(utils.RootPath(), "web/src/lang/backend")
			expected := []string{
				filepath.Join(expectedBase, "en", tc.enLanguage),
				filepath.Join(expectedBase, "zh-cn", tc.zhCNLanguage),
			}
			if manifest.Generated[0] != expected[0] {
				t.Fatalf("unexpected en language path: %s, want %s", manifest.Generated[0], expected[0])
			}
			if manifest.Generated[1] != expected[1] {
				t.Fatalf("unexpected zh-cn language path: %s, want %s", manifest.Generated[1], expected[1])
			}
		})
	}
}

func TestCanonicalManifestLangPath(t *testing.T) {
	root := utils.RootPath()
	cases := []struct {
		name, input, want string
	}{
		{"legacy nested", filepath.Join(root, "web/src/lang/backend/country/en/language.ts"), filepath.Join(root, "web/src/lang/backend/en/country/language.ts")},
		{"new nested", filepath.Join(root, "web/src/lang/backend/en/country/language.ts"), filepath.Join(root, "web/src/lang/backend/en/country/language.ts")},
		{"module named en", filepath.Join(root, "web/src/lang/backend/en/en/language.ts"), filepath.Join(root, "web/src/lang/backend/en/en/language.ts")},
		{"non language", filepath.Join(root, "web/src/views/backend/country/en/language.ts"), filepath.Join(root, "web/src/views/backend/country/en/language.ts")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalManifestLangPath(tc.input); got != tc.want {
				t.Fatalf("canonicalManifestLangPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestManifestAllowsCanonicalizesLegacyLanguagePath(t *testing.T) {
	root := utils.RootPath()
	newPath := filepath.Join(root, "web/src/lang/backend/en/country/language.ts")
	oldPath := filepath.Join(root, "web/src/lang/backend/country/en/language.ts")
	log := &model.CrudLog{Table: model.JSON_TABLE{GeneratedFiles: []string{oldPath}}}
	if !manifestAllows(FileManifest{Generated: []string{newPath}}, log) {
		t.Fatal("legacy language manifest should match locale-first path")
	}
	nonLangOld := filepath.Join(root, "web/src/views/backend/old/country/language.ts")
	nonLangNew := filepath.Join(root, "web/src/views/backend/new/country/language.ts")
	if manifestAllows(FileManifest{Generated: []string{nonLangNew}}, &model.CrudLog{Table: model.JSON_TABLE{GeneratedFiles: []string{nonLangOld}}}) {
		t.Fatal("non-language path migration should remain rejected")
	}
}

func TestHistoricalDeleteManifestCanonicalizesLegacyLanguagePath(t *testing.T) {
	root := utils.RootPath()
	newPath := filepath.Join(root, "web/src/lang/backend/en/.crud-helper-delete/country.ts")
	oldPath := filepath.Join(root, "web/src/lang/backend/.crud-helper-delete/en/country.ts")
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("export default {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Join(root, "web/src/lang/backend/en/.crud-helper-delete"))

	manifest, err := historicalDeleteManifest(FileManifest{}, model.Table{Manifest: &model.CRUDFileManifest{Generated: []string{oldPath}}})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareDeleteManifest(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Generated) != 1 || prepared.Generated[0] != newPath {
		t.Fatalf("historical delete did not resolve new language path: %+v", prepared)
	}
}

func TestHistoricalManifestPreservesGeneratedProviderClassification(t *testing.T) {
	provider := filepath.Join(utils.RootPath(), "app", "admin", "model", "relation", "provider.go")
	current := FileManifest{Shared: []string{provider}}
	historical := model.Table{Manifest: &model.CRUDFileManifest{Generated: []string{provider}}}
	manifest, err := historicalDeleteManifest(current, historical)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Generated) != 1 || manifest.Generated[0] != provider || len(manifest.Shared) != 0 {
		t.Fatalf("historical classification changed: %+v", manifest)
	}
}

func TestPrepareDeleteManifestSkipsMissingGeneratedButRequiresShared(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "app", "admin", "model", ".crud-helper-test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	generated := filepath.Join(dir, "missing.go")
	shared := filepath.Join(dir, "provider.go")
	if err := os.WriteFile(shared, []byte("package model"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest, err := prepareDeleteManifest(FileManifest{Generated: []string{generated}, Shared: []string{shared}})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Generated) != 0 || len(manifest.Shared) != 1 {
		t.Fatalf("unexpected prepared manifest: %+v", manifest)
	}
	if _, err := prepareDeleteManifest(FileManifest{Shared: []string{generated}}); err == nil {
		t.Fatal("missing shared file should fail deletion")
	}
}

func TestHistoricalManifestRejectsPathOutsideAllowedRoots(t *testing.T) {
	for _, path := range []string{"../../etc/passwd", "/tmp/evil.go"} {
		if _, err := historicalDeleteManifest(FileManifest{}, model.Table{Manifest: &model.CRUDFileManifest{Generated: []string{path}}}); err == nil {
			t.Errorf("historical path %q was accepted", path)
		}
	}
}

func TestHistoricalManifestEnforcesGeneratedAndSharedPathClasses(t *testing.T) {
	root := utils.RootPath()
	validGenerated := filepath.Join(root, "app", "admin", "model", "orders.go")
	validShared := filepath.Join(root, "app", "admin", "model", "provider.go")
	if _, err := historicalDeleteManifest(FileManifest{}, model.Table{Manifest: &model.CRUDFileManifest{
		Generated: []string{validGenerated},
		Shared:    []string{validShared},
	}}); err != nil {
		t.Fatalf("valid historical manifest was rejected: %v", err)
	}

	for _, manifest := range []*model.CRUDFileManifest{
		{Generated: []string{filepath.Join(root, "app", "middleware", "security.go")}},
		{Shared: []string{filepath.Join(root, "app", "admin", "model", "admin.go")}},
		{Shared: []string{filepath.Join(root, "router", "unexpected.go")}},
	} {
		if _, err := historicalDeleteManifest(FileManifest{}, model.Table{Manifest: manifest}); err == nil {
			t.Errorf("historical manifest path class was accepted: %+v", manifest)
		}
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

func TestWireErrorIncludesOutput(t *testing.T) {
	err := formatWireError(errors.New("exit status 1"), []byte("wire: undefined provider\n"))
	if !strings.Contains(err.Error(), "undefined provider") {
		t.Fatalf("wire error omitted diagnostic output: %v", err)
	}
}
