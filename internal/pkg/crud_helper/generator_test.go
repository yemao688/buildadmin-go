package crud_helper

import (
	model "buildadmin-go/internal/admin/repository"
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/util"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateFromSpecRejectsProtectedTableBeforeDependencies(t *testing.T) {
	_, err := GenerateFromSpec(nil, nil, GenerateOptions{Table: crudmodel.Table{Name: "admin"}})
	if err == nil || err.Error() != `crud generation is forbidden for protected table "admin"` {
		t.Fatalf("error = %v", err)
	}
}

func TestDeleteQuarantinePathIsReusableByService(t *testing.T) {
	// This exercises the same quarantine primitive used by DeleteFromSpec: a
	// failure before commit restores every manifest member.
	assertQuarantineRestore(t)
}

func TestNormalizeGenerationType(t *testing.T) {
	cases := []struct {
		in, rebuild, want string
	}{
		{"create", "", "create"},
		{"create", "Yes", "create"},
		{"alter", "", "alter"},
		{"log", "Yes", "create"},
		{"log", "No", "alter"},
		{"log", "", "alter"},
		{"db", "Yes", "create"},
		{"db", "", "alter"},
		{"sql", "Yes", "create"},
		{"sql", "", "alter"},
	}
	for _, tc := range cases {
		if got := normalizeGenerationType(tc.in, tc.rebuild); got != tc.want {
			t.Fatalf("normalizeGenerationType(%q, %q) = %q, want %q", tc.in, tc.rebuild, got, tc.want)
		}
	}
}

func TestHandlerTemplateDeleteUsesSuccessMessage(t *testing.T) {
	deleteStart := strings.Index(handlerTemp, "func (h *{{.ClassName}}Handler) Del(ctx *gin.Context) {")
	if deleteStart < 0 {
		t.Fatal("generated handler template is missing delete handler")
	}
	deleteTemplate := handlerTemp[deleteStart:]
	if !strings.Contains(deleteTemplate, `response.SuccessWithMessage(ctx, "Deleted successfully")`) {
		t.Fatal("generated delete handler must return the aligned success message")
	}
	if strings.Contains(deleteTemplate, `response.Success(ctx, "")`) {
		t.Fatal("generated delete handler must not use an empty success response")
	}
}

// 对齐上游:type=create 对已存在的数据表直接删表重建(前端 generateCheck
// 已弹窗确认),服务端仅校验生成类型本身。
func TestValidateGenerationModeAllowsCreateOnExistingTable(t *testing.T) {
	if err := validateGenerationMode("create"); err != nil {
		t.Fatal(err)
	}
	if err := validateGenerationMode("alter"); err != nil {
		t.Fatal(err)
	}
	if err := validateGenerationMode("log"); err == nil {
		t.Fatal("unsupported generation type must be rejected")
	}
}

func TestAlterChangesOnlyAddAndModify(t *testing.T) {
	changes := deriveAlterChanges([]model.Column{{COLUMN_NAME: "id"}, {COLUMN_NAME: "legacy"}}, []crudmodel.Field{{Name: "id"}, {Name: "name"}})
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
	log := &crudmodel.Log{Table: crudmodel.JSON_TABLE{GeneratedFiles: []string{path}}}
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
			manifest, err := BuildFileManifest(crudmodel.Table{
				Name:           tc.name,
				ModelFile:      "internal/admin/model/" + tc.name + ".go",
				ControllerFile: "internal/admin/handler/" + tc.name + ".go",
				WebViewsDir:    tc.viewsPath,
			})
			if err != nil {
				t.Fatal(err)
			}
			expectedBase := filepath.Join(util.RootPath(), "web/src/lang/backend")
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

func TestBuildFileManifestUsesRegistrarOutputAndSharedSet(t *testing.T) {
	root := util.RootPath()
	manifest, err := BuildFileManifest(crudmodel.Table{
		Name:           "country_language_content",
		ModelFile:      "internal/model/country_language_content.go",
		ControllerFile: "internal/admin/handler/country_language_content.go",
		WebViewsDir:    "web/src/views/backend/country/languageContent",
	})
	if err != nil {
		t.Fatal(err)
	}
	registrar := filepath.Join(root, "internal/admin/router/country_language_content.go")
	if !containsPath(manifest.Generated, registrar) {
		t.Fatalf("registrar file missing from generated manifest: %+v", manifest.Generated)
	}
	if containsPath(manifest.Generated, filepath.Join(root, "internal/admin/handler/country_language_content_route.go")) {
		t.Fatalf("handler _route.go must not be emitted in the flat layout: %+v", manifest.Generated)
	}
	routerProvider := filepath.Join(root, "internal/admin/router/provider.go")
	if !containsPath(manifest.Shared, routerProvider) {
		t.Fatalf("router provider missing from shared manifest: %+v", manifest.Shared)
	}
	registrarSet := filepath.Join(root, "internal/router/registrar_set.go")
	if containsPath(manifest.Shared, registrarSet) {
		t.Fatalf("api registrar_set.go must not be in the flat shared manifest: %+v", manifest.Shared)
	}
	if containsPath(manifest.Shared, filepath.Join(root, "cmd/server/wire.go")) {
		t.Fatalf("wire.go must not be in the flat shared manifest: %+v", manifest.Shared)
	}
}

func TestBuildFileManifestNormalizesPathSeparators(t *testing.T) {
	forward := crudmodel.Table{
		Name: "country_language_content", ModelFile: "internal/admin/model/country/languageContent.go",
		ControllerFile: "internal/admin/handler/country/languageContent.go", WebViewsDir: "web/src/views/backend/country/languageContent",
	}
	backslash := forward
	backslash.ModelFile = `internal\admin\model\country\languageContent.go`
	backslash.ControllerFile = `internal\admin\handler\country\languageContent.go`
	backslash.WebViewsDir = `web\src\views\backend\country\languageContent`
	one, err := BuildFileManifest(forward)
	if err != nil {
		t.Fatal(err)
	}
	two, err := BuildFileManifest(backslash)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(one.Generated, "\n") != strings.Join(two.Generated, "\n") || strings.Join(one.Shared, "\n") != strings.Join(two.Shared, "\n") {
		t.Fatalf("separator manifests differ:\nforward=%+v\nbackslash=%+v", one, two)
	}
}

func TestCanonicalManifestLangPath(t *testing.T) {
	root := util.RootPath()
	cases := []struct {
		name, input, want string
	}{
		{"locale first", filepath.Join(root, "web/src/lang/backend/en/country/language.ts"), filepath.Join(root, "web/src/lang/backend/en/country/language.ts")},
		{"module named en", filepath.Join(root, "web/src/lang/backend/en/en/language.ts"), filepath.Join(root, "web/src/lang/backend/en/en/language.ts")},
		{"non language", filepath.Join(root, "web/src/views/backend/country/en/language.ts"), filepath.Join(root, "web/src/views/backend/country/en/language.ts")},
		{"legacy locale second passthrough", filepath.Join(root, "web/src/lang/backend/country/en/language.ts"), filepath.Join(root, "web/src/lang/backend/country/en/language.ts")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalManifestLangPath(tc.input); got != tc.want {
				t.Fatalf("canonicalManifestLangPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
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
	if strings.HasPrefix(err.Error(), "wire: wire:") {
		t.Fatalf("wire error should not duplicate stage prefix: %v", err)
	}
	if !strings.Contains(err.Error(), "undefined provider") {
		t.Fatalf("wire error omitted diagnostic output: %v", err)
	}
}

func TestWireErrorWithoutOutputReturnsOriginalError(t *testing.T) {
	base := errors.New("signal: interrupt")
	err := formatWireError(base, nil)
	if !errors.Is(err, base) {
		t.Fatalf("wire error should preserve original cause: %v", err)
	}
	if err.Error() != "signal: interrupt" {
		t.Fatalf("wire error without output = %q", err.Error())
	}
}

func TestParseDeleteGoFilesReportsPathAndLine(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "provider.go")
	invalid := filepath.Join(dir, "registrar_set.go")
	if err := os.WriteFile(valid, []byte("package provider\n\nvar ProviderSet = struct{}{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalid, []byte("package router\n\nvar ProviderSet = []int{\n\t1,\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := parseDeleteGoFiles(valid, invalid)
	if err == nil {
		t.Fatal("invalid provider output should fail the parse guard")
	}
	if !strings.Contains(err.Error(), invalid) || !strings.Contains(err.Error(), ":4:") {
		t.Fatalf("parse guard error lacks path/line: %v", err)
	}
}
