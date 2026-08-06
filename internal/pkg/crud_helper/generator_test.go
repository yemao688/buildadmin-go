package crud_helper

import (
	model "buildadmin-go/internal/admin/repository"
	entity "buildadmin-go/internal/model"
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/util"
	"errors"
	"os"
	"path/filepath"
	"slices"
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

// TestSkipManifestPathsAndFilter 验证 skip 产物路径推导与 manifest 过滤：
// skip-frontend 剔除 4 个 web 产物，skip-repo 剔除 repository 文件与所在包
// provider.go；过滤后 Generated/Shared 不含被跳过路径，且 manifestAllows 的
// skipped 集合与 filterManifestPaths 使用的集合同源。
func TestSkipManifestPathsAndFilter(t *testing.T) {
	table := crudmodel.Table{Name: "orders", WebViewsDir: "web/src/views/backend/order/orders"}
	skipped := skipManifestPaths(table, true, true)
	if len(skipped) != 6 { // 4 web + repository + repository provider
		t.Fatalf("skipManifestPaths(frontend+repo) = %d paths, want 6", len(skipped))
	}

	views := ParseWebDirNameData(table.Name, "views", table.WebViewsDir)
	lang := ParseWebDirNameData(table.Name, "lang", table.WebViewsDir)
	for _, p := range []string{
		filepath.Join(util.RootPath(), lang.LangFile("en")),
		filepath.Join(util.RootPath(), lang.LangFile("zh-cn")),
		filepath.Join(util.RootPath(), views.Views, "index.vue"),
		filepath.Join(util.RootPath(), views.Views, "popupForm.vue"),
	} {
		if !skipped[filepath.Clean(p)] {
			t.Fatalf("skip-frontend should skip %s", p)
		}
	}
	repo, err := ParseRepositoryNameData(table.Name, "")
	if err != nil {
		t.Fatal(err)
	}
	if !skipped[filepath.Clean(repo.ParseFile)] {
		t.Fatalf("skip-repo should skip repository file %s", repo.ParseFile)
	}
	if !skipped[filepath.Clean(filepath.Join(util.RootPath(), repo.RootFileName, "provider.go"))] {
		t.Fatal("skip-repo should skip repository provider.go")
	}

	// 只 skip-frontend：web 产物剔除、repo 保留。
	onlyWeb := skipManifestPaths(table, true, false)
	if len(onlyWeb) != 4 {
		t.Fatalf("skipManifestPaths(frontend only) = %d paths, want 4", len(onlyWeb))
	}
	if onlyWeb[filepath.Clean(repo.ParseFile)] {
		t.Fatal("repo file must not be skipped when only skip-frontend is set")
	}

	// filterManifestPaths 与 skipManifestPaths 同源消费。
	manifest := FileManifest{
		Generated: []string{
			filepath.Join(util.RootPath(), lang.LangFile("en")),
			filepath.Join(util.RootPath(), views.Views, "index.vue"),
			filepath.Join(util.RootPath(), "internal", "model", "orders.go"),
			repo.ParseFile,
		},
		Shared: []string{filepath.Join(util.RootPath(), repo.RootFileName, "provider.go")},
	}
	filtered := filterManifestPaths(manifest, onlyWeb)
	if len(filtered.Generated) != 2 { // model.go + repository 保留
		t.Fatalf("filtered Generated = %d paths, want 2", len(filtered.Generated))
	}
	if slices.Contains(filtered.Generated, filepath.Join(util.RootPath(), lang.LangFile("en"))) {
		t.Fatal("filtered manifest must drop lang en")
	}
	if slices.Contains(filtered.Generated, filepath.Join(util.RootPath(), views.Views, "index.vue")) {
		t.Fatal("filtered manifest must drop index.vue")
	}
	if !slices.Contains(filtered.Generated, filepath.Join(util.RootPath(), "internal", "model", "orders.go")) {
		t.Fatal("filtered manifest must keep entity file")
	}
	if !slices.Contains(filtered.Generated, repo.ParseFile) {
		t.Fatal("filtered manifest must keep repository file")
	}
	if len(filtered.Shared) != 1 {
		t.Fatalf("filtered Shared = %d paths, want 1", len(filtered.Shared))
	}
}

func TestManifestAllowsOnlyLatestSuccessfulTargets(t *testing.T) {
	path := t.TempDir() + "/model.go"
	manifest := FileManifest{Generated: []string{path}}
	if !manifestAllows(manifest, nil, nil) {
		t.Fatal("first generation without a success manifest should be allowed")
	}
	if err := os.WriteFile(path, []byte("package model"), 0644); err != nil {
		t.Fatal(err)
	}
	if manifestAllows(manifest, nil, nil) {
		t.Fatal("first generation must reject an existing target")
	}
	handlerPath := path + ".handler"
	if err := os.WriteFile(handlerPath, []byte("package handler"), 0644); err != nil {
		t.Fatal(err)
	}
	if manifestAllows(FileManifest{Generated: []string{path, handlerPath}}, nil, nil) {
		t.Fatal("first generation must reject existing model and handler targets")
	}
	log := &entity.Log{Table: crudmodel.JSON_TABLE{GeneratedFiles: []string{path}}}
	if !manifestAllows(manifest, log, nil) {
		t.Fatal("latest success manifest should allow its own target")
	}
	// 路径迁移（替换形态）：上次 {path}，本次 {path.new}，两清单互不包含
	// → 旧文件会残留，拒绝。注意"新增"形态（{path, path.new} 相对 {path}）
	// 是合法演进（skip 恢复、remoteSelect 关联实体），双向子集语义允许它。
	if manifestAllows(FileManifest{Generated: []string{path + ".new"}}, log, nil) {
		t.Fatal("manifest path migration should be rejected")
	}
}

// TestManifestAllowsSkippedSubset 验证双向子集语义：
//   - skip 方向（全量 → skip）：本次清单是上次清单剔除本次跳过路径后的
//     等长子集，允许（跳过后重新生成不被拒）；
//   - 恢复方向（skip → 全量）：上次清单是本次清单的子集，允许（被跳过产物
//     重新纳入，无需先 crud:delete）；
//   - 路径漂移（两清单互不包含）始终拒绝。
func TestManifestAllowsSkippedSubset(t *testing.T) {
	path := t.TempDir() + "/model.go"
	skipped := map[string]bool{filepath.Clean(path): true}
	log := &entity.Log{Table: crudmodel.JSON_TABLE{GeneratedFiles: []string{path}}}
	// 上次全量生成（含 model.go），本次跳过它：manifest 不含该路径，允许。
	if !manifestAllows(FileManifest{Generated: nil}, log, skipped) {
		t.Fatal("skip subset of previous manifest should be allowed")
	}
	// 新增路径仍拒绝（即使有跳过集合）：current 与 previous 互不包含。
	if manifestAllows(FileManifest{Generated: []string{path + ".new"}}, log, skipped) {
		t.Fatal("new path must still be rejected under skip mode")
	}

	// 恢复方向：上次 skip 记录（子集），本次全量（含被跳过路径）→ 允许。
	fullLog := &entity.Log{Table: crudmodel.JSON_TABLE{GeneratedFiles: []string{path + ".skip-recorded"}}}
	backToFull := FileManifest{Generated: []string{path, path + ".skip-recorded"}}
	if !manifestAllows(backToFull, fullLog, nil) {
		t.Fatal("restoring previously skipped paths (previous ⊆ current) should be allowed")
	}
	// 恢复方向的漂移防护：上次记录路径不在本次清单 → 拒绝。
	if manifestAllows(FileManifest{Generated: []string{path + ".elsewhere"}}, fullLog, nil) {
		t.Fatal("path drift must be rejected even under restore direction")
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
