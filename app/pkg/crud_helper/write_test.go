package crud_helper

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAtomicCapabilitiesAreRemovedWithRouter(t *testing.T) {
	const marker = "\t} {\n\t\tmiddleware.RegisterAtomicRoute(capability)"
	original := "prefix\n" + marker + "\nsuffix\n"
	name := "aiGateDemo"
	injected := injectAtomicCapabilities(original, name, marker)
	if !strings.Contains(injected, `Route: "aiGateDemo/add"`) {
		t.Fatal("atomic capabilities were not injected")
	}
	removed := removeAtomicCapabilities(injected, name)
	if removed != original {
		t.Fatalf("router content was not restored after removal:\n%s", removed)
	}
	if removeAtomicCapabilities(original, name) != original {
		t.Fatal("removing absent capabilities must be idempotent")
	}
}

func TestProviderEntryRoundTrip(t *testing.T) {
	original := "package model\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewFooModel,\n\n\tNewBarModel,\n)\n"
	lastIndex := strings.LastIndex(original, ")")
	added := original[:lastIndex] + "\tNewTestModel,\n)"
	if strings.Contains(added, ",\n\n\n\tNewTestModel") {
		t.Fatalf("insert produced a double blank line:\n%s", added)
	}
	removed, err := removeProviderEntry(added, "TestModel")
	if err != nil {
		t.Fatal(err)
	}
	if removed != original {
		t.Fatalf("provider round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", removed, original)
	}
}

func TestRouterEntryRoundTrip(t *testing.T) {
	original := "package router\n\nfunc InitRouter(\n\tcountryCurrencyHandler *admin.CountryCurrencyHandler,\n) *gin.Engine {\n\trouter := gin.New()\n\tadmin.CollectRoutes(router)\n\n\tadminRouter.GET(\"countryCurrency/index\", countryCurrencyHandler.Index)\n}\n"
	added := insertRouterEntry(original, "Test")
	if !strings.Contains(added, "testHandler *admin.TestHandler,") {
		t.Fatalf("router entry was not injected:\n%s", added)
	}
	if !strings.Contains(added, `adminRouter.POST("test/sortable", testHandler.Sortable)`) {
		t.Fatalf("sortable route was not injected:\n%s", added)
	}
	removed, err := removeRouterEntry(added, "Test")
	if err != nil {
		t.Fatal(err)
	}
	if removed != original {
		t.Fatalf("router round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", removed, original)
	}
}

func TestWriteProviderCreatesMissingScaffold(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "app", "admin", "model", "provider_scaffold_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := writeProvider("app/admin/model/provider_scaffold_test", "OwnerModel"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "provider.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "ProviderSet") || !strings.Contains(string(content), "NewOwnerModel") {
		t.Fatalf("provider scaffold was not injected: %s", content)
	}
}

func TestProviderWriteRoundTripPreservesEOFConvention(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "app", "admin", "model", "provider_eof_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	provider := filepath.Join(dir, "provider.go")
	base := "package provider_eof_test\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewExistingModel,\n)"
	canonical := base + "\n"
	for _, original := range []string{base, canonical, canonical + "\n\n"} {
		t.Run(strings.ReplaceAll(strconv.Quote(original), "\\n", "\\\n"), func(t *testing.T) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(provider, []byte(original), 0644); err != nil {
				t.Fatal(err)
			}
			if err := writeProvider("app/admin/model/provider_eof_test", "OwnerModel"); err != nil {
				t.Fatal(err)
			}
			added, err := os.ReadFile(provider)
			if err != nil {
				t.Fatal(err)
			}
			assertExactlyOneTrailingLF(t, string(added))
			if !strings.Contains(string(added), "\tNewOwnerModel,\n") {
				t.Fatalf("provider entry missing after add: %q", added)
			}
			if err := RemoveProvider("app/admin/model/provider_eof_test", "OwnerModel"); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(provider)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != canonical {
				t.Fatalf("first remove did not normalize provider: got %q, want %q", got, canonical)
			}
			for cycle := 0; cycle < 3; cycle++ {
				if err := writeProvider("app/admin/model/provider_eof_test", "OwnerModel"); err != nil {
					t.Fatal(err)
				}
				if err := RemoveProvider("app/admin/model/provider_eof_test", "OwnerModel"); err != nil {
					t.Fatal(err)
				}
				got, err := os.ReadFile(provider)
				if err != nil {
					t.Fatal(err)
				}
				assertExactlyOneTrailingLF(t, string(got))
				if string(got) != canonical {
					t.Fatalf("cycle %d changed provider EOF/content: got %q, want %q", cycle, got, canonical)
				}
			}
		})
	}
}

func TestRemoveRouterEntryNormalizesSharedGoEOF(t *testing.T) {
	original := "package router\n\nfunc InitRouter(\n\ttestHandler *admin.TestHandler,\n) *gin.Engine {\n\trouter := gin.New()\n\tadmin.CollectRoutes(router)\n\n\tadminRouter.GET(\"test/index\", testHandler.Index)\n\tadminRouter.POST(\"test/add\", testHandler.Add)\n\tadminRouter.GET(\"test/edit\", testHandler.One)\n\tadminRouter.POST(\"test/edit\", testHandler.Edit)\n\tadminRouter.DELETE(\"test/del\", testHandler.Del)\n\tadminRouter.POST(\"test/sortable\", testHandler.Sortable)\n}\n"
	trimmed := strings.TrimSuffix(original, "\n")
	removed, err := removeRouterEntry(trimmed, "Test")
	if err != nil {
		t.Fatal(err)
	}
	assertExactlyOneTrailingLF(t, removed)
	if removed != "package router\n\nfunc InitRouter() *gin.Engine {\n\trouter := gin.New()\n\tadmin.CollectRoutes(router)\n\n}\n" {
		t.Fatalf("shared router content did not normalize canonically: %q", removed)
	}
	removedAgain, err := removeRouterEntry(removed, "Test")
	if err != nil {
		t.Fatal(err)
	}
	assertExactlyOneTrailingLF(t, removedAgain)
	if removedAgain != removed {
		t.Fatalf("shared router removal is not stable: got %q, want %q", removedAgain, removed)
	}
}

func TestRemoveAssociatedModelProviderEntries(t *testing.T) {
	dir := filepath.Join(utils.RootPath(), "app", "admin", "model", "assoc_provider_test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	provider := filepath.Join(dir, "provider.go")
	content := "package assoc_provider_test\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewChildModel,\n\tNewAssocModel,\n)\n"
	if err := os.WriteFile(provider, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	fields := []model.Field{{Form: model.FormAttr{RemoteTable: "assoc", RemoteModel: "app/admin/model/assoc_provider_test/Assoc.go", RelationFields: "name"}}}
	manifest := FileManifest{
		Generated: []string{filepath.Join(utils.RootPath(), "app", "admin", "model", "assoc_provider_test", "Assoc.go")},
		Shared:    []string{provider},
	}
	if err := removeAssociatedModelProviders(fields, manifest); err != nil {
		t.Fatal(err)
	}
	updated, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(updated), "NewAssocModel") || !strings.Contains(string(updated), "NewChildModel") {
		t.Fatalf("associated provider entry removal incorrect: %s", updated)
	}
}

// 关联既有核心模型(如 ba_admin)时,不得从共享 provider.go 中移除其注册,
// 否则 wire 将因缺少核心模型 provider 而失败。
func TestRemoveAssociatedModelProvidersKeepsCoreModel(t *testing.T) {
	provider := filepath.Join(utils.RootPath(), "app", "admin", "model", "provider.go")
	before, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	fields := []model.Field{{Form: model.FormAttr{RemoteTable: "ba_admin", RemoteModel: "app/admin/model/admin.go", RelationFields: "username"}}}
	manifest := FileManifest{
		Generated: []string{filepath.Join(utils.RootPath(), "app", "admin", "model", "test.go")},
		Shared:    []string{provider},
	}
	if err := removeAssociatedModelProviders(fields, manifest); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("core model provider.go must remain untouched")
	}
}

func TestRewriteFlexNumericParamFields(t *testing.T) {
	input := "type DemoParam struct {\n\tEnabled bool `json:\"enabled\"`\n\tCount int32 `json:\"count\"`\n\tTotal int64 `json:\"total\"`\n\tRate float64 `json:\"rate\"`\n\tName string `json:\"name\"`\n}\n"
	want := "type DemoParam struct {\n\tEnabled validate.FlexBool `json:\"enabled\"`\n\tCount validate.FlexInt32 `json:\"count\"`\n\tTotal validate.FlexInt64 `json:\"total\"`\n\tRate validate.FlexFloat64 `json:\"rate\"`\n\tName string `json:\"name\"`\n}\n"
	if got := rewriteFlexNumericParamFields(input, nil); got != want {
		t.Fatalf("unexpected rewritten fields:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestRewriteHandlerParamFieldsUsesDesignTypeOverrides(t *testing.T) {
	input := "type DemoParam struct {\n" +
		"\tCount int32 `json:\"count\"`\n" +
		"\tCheckbox []string `json:\"feature_flags\"`\n" +
		"\tSelects []string `json:\"category_values\"`\n" +
		"\tRemoteSelects []string `json:\"reviewer_ids\"`\n" +
		"\tCity string `json:\"region_city\"`\n" +
		"\tImages []string `json:\"gallery_images\"`\n" +
		"\tFiles []string `json:\"attachments_files\"`\n" +
		"\tExtra []string `json:\"extra_data\"`\n" +
		"\tScheduled string `json:\"scheduled_at\"`\n" +
		"\tPublished string `json:\"published_on\"`\n" +
		"\tAt string `json:\"published_at\"`\n" +
		"\tCreated int64 `json:\"created_at\"`\n" +
		"\tTitle string `json:\"title_string\"`\n" +
		"\tPlain string `json:\"plain_text\"`\n" +
		"}\n"
	overrides := map[string]string{
		"count":             "validate.CustomInt",
		"feature_flags":     "validate.CommaJoined",
		"category_values":   "validate.CommaJoined",
		"reviewer_ids":      "validate.CommaJoined",
		"region_city":       "validate.CommaJoined",
		"gallery_images":    "validate.CommaJoined",
		"attachments_files": "validate.CommaJoined",
		"extra_data":        "validate.KeyValueArray",
		"scheduled_at":      "validate.FlexDateTime",
		"published_on":      "validate.FlexDate",
		"published_at":      "validate.FlexClock",
		"created_at":        "validate.FlexUnixTime",
		"title_string":      "validate.CustomString",
	}
	wantTypes := map[string]string{
		"Count":         "validate.CustomInt",
		"Checkbox":      "validate.CommaJoined",
		"Selects":       "validate.CommaJoined",
		"RemoteSelects": "validate.CommaJoined",
		"City":          "validate.CommaJoined",
		"Images":        "validate.CommaJoined",
		"Files":         "validate.CommaJoined",
		"Extra":         "validate.KeyValueArray",
		"Scheduled":     "validate.FlexDateTime",
		"Published":     "validate.FlexDate",
		"At":            "validate.FlexClock",
		"Created":       "validate.FlexUnixTime",
		"Title":         "validate.CustomString",
	}
	got := rewriteFlexNumericParamFields(input, overrides)
	for field, typeName := range wantTypes {
		if !strings.Contains(got, field+" "+typeName+" `json:") {
			t.Errorf("%s was not rewritten to %s:\n%s", field, typeName, got)
		}
	}
	if strings.Contains(got, "title_string string") {
		t.Fatal("override matching must use the JSON name, not the Go field type")
	}
	if !strings.Contains(got, "Plain string `json:\"plain_text\"`") {
		t.Fatalf("ordinary string field was changed:\n%s", got)
	}
}

func TestHandlerParamTypeOverridesFromAnalysedFields(t *testing.T) {
	cases := []struct {
		name       string
		designType string
		dataType   string
		typeName   string
		length     int
		defaultTyp string
		defaultVal string
		want       string
	}{
		{name: "switch tinyint length", designType: "switch", typeName: "tinyint", length: 1, want: "validate.FlexBool"},
		{name: "radio tinyint data type", designType: "radio", typeName: "tinyint", dataType: "tinyint(1)", want: "validate.FlexBool"},
		{name: "year design type", designType: "year", typeName: "year", dataType: "year", want: "validate.FlexYear"},
		{name: "tinyint input default remains numeric", designType: "number", typeName: "tinyint", defaultTyp: "INPUT", defaultVal: "1", want: ""},
		{name: "char one is not bool", designType: "radio", typeName: "char", dataType: "char(1)", length: 1, want: ""},
		{name: "checkbox", designType: "checkbox", dataType: "set", want: "validate.CommaJoined"},
		{name: "selects", designType: "selects", dataType: "set", want: "validate.CommaJoined"},
		{name: "remoteSelects", designType: "remoteSelects", dataType: "varchar", want: "validate.CommaJoined"},
		{name: "city", designType: "city", dataType: "varchar", want: "validate.CommaJoined"},
		{name: "images", designType: "images", dataType: "text", want: "validate.CommaJoined"},
		{name: "files", designType: "files", dataType: "text", want: "validate.CommaJoined"},
		{name: "array", designType: "array", dataType: "text", want: "validate.KeyValueArray"},
		{name: "datetime", designType: "datetime", dataType: "datetime", want: "validate.FlexDateTime"},
		{name: "date", designType: "date", dataType: "date", want: "validate.FlexDate"},
		{name: "time", designType: "time", dataType: "time", want: "validate.FlexClock"},
		{name: "create_time", designType: "timestamp", dataType: "bigint", want: ""},
		{name: "end_time", designType: "timestamp", dataType: "bigint", want: "validate.FlexFormattedUnixTime"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field := analyseField(model.Field{Name: tc.name, DesignType: tc.designType, Type: tc.typeName, DataType: tc.dataType, Length: tc.length, DefaultType: tc.defaultTyp, Default: tc.defaultVal})
			got := buildHandlerParamTypeOverrides([]model.Field{field})
			if got[tc.name] != tc.want {
				t.Fatalf("override = %q, want %q (analysed field: %+v)", got[tc.name], tc.want, field)
			}
		})
	}
}

func TestModelFieldTypeOverridesMatchStorageContracts(t *testing.T) {
	fields := []model.Field{
		{Name: "enabled", Type: "tinyint", Length: 1, DesignType: "switch"},
		{Name: "visible", Type: "tinyint", DataType: "tinyint(1)", DesignType: "radio"},
		{Name: "published_year", Type: "year", DataType: "year", DesignType: "year"},
		{Name: "char_flag", Type: "char", Length: 1, DesignType: "radio"},
		{Name: "flags", DataType: "set", DesignType: "checkbox"},
		{Name: "options", DataType: "text", DesignType: "array"},
		{Name: "created_at", DataType: "datetime", DesignType: "datetime"},
		{Name: "published_at", DataType: "timestamp", DesignType: "datetime"},
		{Name: "day", DataType: "date", DesignType: "date"},
		{Name: "clock", DataType: "time", DesignType: "time"},
		{Name: "clock_native", DataType: "TIME", DesignType: "string"},
		{Name: "create_time", DataType: "bigint", DesignType: "timestamp"},
		{Name: "unix_at", DataType: "bigint", DesignType: "timestamp"},
		{Name: "mismatch", DataType: "datetime", DesignType: "string"},
	}
	got := buildModelFieldTypeOverrides(fields)
	want := map[string]string{
		"enabled":        "validate.FlexBool",
		"visible":        "validate.FlexBool",
		"published_year": "validate.FlexYear",
		"flags":          "validate.CommaJoined",
		"options":        "validate.KeyValueArray",
		"created_at":     "validate.FlexDateTime",
		"published_at":   "validate.FlexDateTime",
		"day":            "validate.FlexDate",
		"clock":          "validate.FlexClock",
		"clock_native":   "string",

		"unix_at": "validate.FlexFormattedUnixTime",
	}
	if len(got) != len(want) {
		t.Fatalf("overrides = %#v, want %#v", got, want)
	}
	for name, typeName := range want {
		if got[name] != typeName {
			t.Errorf("%s override = %q, want %q", name, got[name], typeName)
		}
	}
	if _, ok := got["mismatch"]; ok {
		t.Fatal("unrelated mismatched design type was overridden")
	}
	if _, ok := got["char_flag"]; ok {
		t.Fatal("char(1) must not be treated as boolean storage")
	}
}

func TestRenderHandlerSharesParamTypeForAddAndEdit(t *testing.T) {
	structContent := "type Demo struct {\n\tFeatureFlags []string `json:\"feature_flags\"`\n}\n"
	handlerData := HandlerData{
		Namespace:       "admin",
		ClassName:       "Demo",
		ModelNamespace:  "model",
		ModelImportPath: "go-build-admin/app/admin/model",
		ModelName:       "Demo",
		ModelVar:        "demo",
		PkGoType:        "int32",
		PkJSONName:      "id",
		ParamTypeOverrides: map[string]string{
			"feature_flags": "validate.CommaJoined",
		},
	}
	content, err := renderHandler(handlerData, structContent)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(content, "validate.CommaJoined") != 1 || strings.Count(content, "FeatureFlags validate.CommaJoined") != 1 {
		t.Fatalf("Add/Edit should share one rewritten parameter type:\n%s", content)
	}
	if strings.Count(content, "DemoParam") < 3 {
		t.Fatalf("expected Add and Edit to use the shared parameter struct:\n%s", content)
	}
}

func TestModelTemplateAddsWeighFromIntegerPrimaryKey(t *testing.T) {
	withWeigh := `type Demo struct {
	ID int32 ` + "`json:\"id\"`" + `
	Weigh int32 ` + "`json:\"weigh\"`" + `
}

func TestModelTemplateAddsRowFactory(t *testing.T) {
	data := ModelData{
		Namespace:  "model",
		ClassName:  "Demo",
		ModelVar:   "demo",
		Pk:         "id",
		StructTemp: "type Demo struct{}",
	}
	code, err := renderModel(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "func (s *DemoModel) NewRow() any") || !strings.Contains(code, "return &Demo{}") {
		t.Fatalf("row factory missing from model template:\n%s", code)
	}
}

`
	withoutWeigh := `type Demo struct {
	ID int32 ` + "`json:\"id\"`" + `
}

`
	data := ModelData{Namespace: "model", ClassName: "Demo", ModelVar: "demo", Pk: "id", PkGoField: "ID", StructTemp: withWeigh}
	withCode, err := renderModel(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withCode, "if demo.Weigh == 0") || !strings.Contains(withCode, `Update("weigh", demo.ID)`) || !strings.Contains(withCode, "demo.Weigh = int32(demo.ID)") {
		t.Fatalf("weigh post-insert hook missing:\n%s", withCode)
	}

	data.StructTemp = withoutWeigh
	withoutCode, err := renderModel(data)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(withoutCode, "demo.Weigh") || strings.Contains(withoutCode, `Update("weigh", demo.ID)`) {
		t.Fatalf("weigh post-insert hook emitted without a weigh field:\n%s", withoutCode)
	}
}

func TestWriteGoFileNormalizesFormattingAndEOF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "provider.go")
	if err := writeGoFile(path, "package provider\n\n\nvar ProviderSet = 1\n\n\n"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "package provider\n\nvar ProviderSet = 1\n" {
		t.Fatalf("normalized Go output = %q", content)
	}
	assertExactlyOneTrailingLF(t, string(content))
}

func assertExactlyOneTrailingLF(t *testing.T, content string) {
	t.Helper()
	if !strings.HasSuffix(content, "\n") || strings.HasSuffix(content, "\n\n") {
		t.Fatalf("content does not end with exactly one trailing LF: %q", content)
	}
}
