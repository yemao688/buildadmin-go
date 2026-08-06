package crud_helper

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/util"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRegistrarTemplateRendersAndFormats(t *testing.T) {
	path := filepath.Join(util.RootPath(), "internal", "admin", "handler", "demo_route.go")
	content, err := render(path, registrarTemp, RegistrarData{
		Namespace: "handler",
		ClassName: "Demo",
		RouteName: "demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `CRUDRoutes(g, demoRoute, r.handler)`) {
		t.Fatalf("registrar route call missing:\n%s", content)
	}
	if !strings.Contains(content, `CRUDCapabilities(demoRoute)`) {
		t.Fatalf("registrar capability call missing:\n%s", content)
	}
}

func TestRegistrarTemplateMatchesCountryLanguageShape(t *testing.T) {
	path := filepath.Join(util.RootPath(), "internal", "admin", "router", "country_language.go")
	content, err := render(path, registrarTemp, RegistrarData{
		Namespace: "router",
		ClassName: "CountryLanguage",
		RouteName: "countryLanguage",
		RoutePath: "country.Language",
	})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if content != string(expected) {
		t.Fatalf("registrar template differs from country sample:\n--- got ---\n%s\n--- want ---\n%s", content, expected)
	}
}

func assertParseableGo(t *testing.T, filename, content string) {
	t.Helper()
	if _, err := parser.ParseFile(token.NewFileSet(), filename, content, parser.AllErrors); err != nil {
		t.Fatalf("%s is not parseable: %v\n%s", filename, err, content)
	}
}

func TestWriteProviderCreatesMissingScaffold(t *testing.T) {
	dir := filepath.Join(util.RootPath(), "internal", "admin", "model", "provider_scaffold_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := writeProvider("internal/admin/model/provider_scaffold_test", "OwnerModel"); err != nil {
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

func TestWriteProviderSequentialEntriesOnFreshPackage(t *testing.T) {
	dir := filepath.Join(util.RootPath(), "internal", "admin", "handler", "provider_fresh_seq_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	// 首个条目写入后 gofmt 会把单参数 NewSet 折叠成单行，第二个条目必须仍能合法追加
	if err := writeProvider("internal/admin/handler/provider_fresh_seq_test", "UserHandler"); err != nil {
		t.Fatal(err)
	}
	if err := writeProvider("internal/admin/handler/provider_fresh_seq_test", "UserRegistrar"); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(dir, "provider.go"))
	if err != nil {
		t.Fatal(err)
	}
	assertParseableGo(t, "provider.go", string(content))
	if !strings.Contains(string(content), "\tNewUserHandler,\n") || !strings.Contains(string(content), "\tNewUserRegistrar,\n") {
		t.Fatalf("sequential provider entries missing: %s", content)
	}
	assertExactlyOneTrailingLF(t, string(content))
}

func TestProviderWriteRoundTripPreservesEOFConvention(t *testing.T) {
	dir := filepath.Join(util.RootPath(), "internal", "admin", "model", "provider_eof_test")
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
			if err := writeProvider("internal/admin/model/provider_eof_test", "OwnerModel"); err != nil {
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
			if err := RemoveProvider("internal/admin/model/provider_eof_test", "OwnerModel"); err != nil {
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
				if err := writeProvider("internal/admin/model/provider_eof_test", "OwnerModel"); err != nil {
					t.Fatal(err)
				}
				if err := RemoveProvider("internal/admin/model/provider_eof_test", "OwnerModel"); err != nil {
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

func TestRegistrarProviderWriteRoundTrip(t *testing.T) {
	dir := filepath.Join(util.RootPath(), "internal", "admin", "handler", "registrar_provider_test")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	provider := filepath.Join(dir, "provider.go")
	original := "package registrar_provider_test\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet(\n\tNewExistingHandler,\n)\n"
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(provider, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeProvider("internal/admin/handler/registrar_provider_test", "OwnerRegistrar"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveProvider("internal/admin/handler/registrar_provider_test", "OwnerRegistrar"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(provider)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Fatalf("registrar provider round trip mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, original)
	}
}

func TestRewriteFlexNumericParamFields(t *testing.T) {
	input := "type DemoParam struct {\n\tEnabled bool `json:\"enabled\"`\n\tCount int32 `json:\"count\"`\n\tTotal int64 `json:\"total\"`\n\tRate float64 `json:\"rate\"`\n\tName string `json:\"name\"`\n}\n"
	want := "type DemoParam struct {\n\tEnabled validator.FlexBool `json:\"enabled\"`\n\tCount validator.FlexInt32 `json:\"count\"`\n\tTotal validator.FlexInt64 `json:\"total\"`\n\tRate validator.FlexFloat64 `json:\"rate\"`\n\tName string `json:\"name\"`\n}\n"
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
		"count":             "validator.CustomInt",
		"feature_flags":     "validator.CommaJoined",
		"category_values":   "validator.CommaJoined",
		"reviewer_ids":      "validator.CommaJoined",
		"region_city":       "validator.CommaJoined",
		"gallery_images":    "validator.CommaJoined",
		"attachments_files": "validator.CommaJoined",
		"extra_data":        "validator.KeyValueArray",
		"scheduled_at":      "validator.FlexDateTime",
		"published_on":      "validator.FlexDate",
		"published_at":      "validator.FlexClock",
		"created_at":        "validator.FlexUnixTime",
		"title_string":      "validator.CustomString",
	}
	wantTypes := map[string]string{
		"Count":         "validator.CustomInt",
		"Checkbox":      "validator.CommaJoined",
		"Selects":       "validator.CommaJoined",
		"RemoteSelects": "validator.CommaJoined",
		"City":          "validator.CommaJoined",
		"Images":        "validator.CommaJoined",
		"Files":         "validator.CommaJoined",
		"Extra":         "validator.KeyValueArray",
		"Scheduled":     "validator.FlexDateTime",
		"Published":     "validator.FlexDate",
		"At":            "validator.FlexClock",
		"Created":       "validator.FlexUnixTime",
		"Title":         "validator.CustomString",
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
		{name: "switch tinyint length", designType: "switch", typeName: "tinyint", length: 1, want: "validator.FlexBool"},
		{name: "radio tinyint data type stays numeric", designType: "radio", typeName: "tinyint", dataType: "tinyint(1)", want: ""},
		{name: "year design type", designType: "year", typeName: "year", dataType: "year", want: "validator.FlexYear"},
		{name: "tinyint input default remains numeric", designType: "number", typeName: "tinyint", defaultTyp: "INPUT", defaultVal: "1", want: ""},
		{name: "char one is not bool", designType: "radio", typeName: "char", dataType: "char(1)", length: 1, want: ""},
		{name: "checkbox", designType: "checkbox", dataType: "set", want: "validator.CommaJoined"},
		{name: "selects", designType: "selects", dataType: "set", want: "validator.CommaJoined"},
		{name: "remoteSelects", designType: "remoteSelects", dataType: "varchar", want: "validator.CommaJoined"},
		{name: "city", designType: "city", dataType: "varchar", want: "validator.CommaJoined"},
		{name: "images", designType: "images", dataType: "text", want: "validator.CommaJoined"},
		{name: "files", designType: "files", dataType: "text", want: "validator.CommaJoined"},
		{name: "array", designType: "array", dataType: "text", want: "validator.KeyValueArray"},
		{name: "datetime", designType: "datetime", dataType: "datetime", want: "validator.FlexDateTime"},
		{name: "date", designType: "date", dataType: "date", want: "validator.FlexDate"},
		{name: "time", designType: "time", dataType: "time", want: "validator.FlexClock"},
		{name: "create_time", designType: "timestamp", dataType: "bigint", want: ""},
		{name: "end_time", designType: "timestamp", dataType: "bigint", want: "validator.FlexFormattedUnixTime"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field := analyseField(crudmodel.Field{Name: tc.name, DesignType: tc.designType, Type: tc.typeName, DataType: tc.dataType, Length: tc.length, DefaultType: tc.defaultTyp, Default: tc.defaultVal})
			got := buildHandlerParamTypeOverrides([]crudmodel.Field{field})
			if got[tc.name] != tc.want {
				t.Fatalf("override = %q, want %q (analysed field: %+v)", got[tc.name], tc.want, field)
			}
		})
	}
}

func TestModelFieldTypeOverridesMatchStorageContracts(t *testing.T) {
	fields := []crudmodel.Field{
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
		"enabled":        "validator.FlexBool",
		"published_year": "validator.FlexYear",
		"flags":          "validator.CommaJoined",
		"options":        "validator.KeyValueArray",
		"created_at":     "validator.FlexDateTime",
		"published_at":   "validator.FlexDateTime",
		"day":            "validator.FlexDate",
		"clock":          "validator.FlexClock",
		"clock_native":   "string",

		"unix_at": "validator.FlexFormattedUnixTime",
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
		ModelImportPath: "buildadmin-go/internal/model",
		ModelName:       "Demo",
		ModelVar:        "demo",
		PkGoType:        "int32",
		PkJSONName:      "id",
		DTOQualifier:    "dto.",
		ParamTypeOverrides: map[string]string{
			"feature_flags": "validator.CommaJoined",
		},
	}
	// 参数类型改写发生在 DTO 文件（buildParamStruct + renderDTO）
	paramStruct := buildParamStruct(structContent, handlerData)
	if !strings.Contains(paramStruct, "FeatureFlags validator.CommaJoined") {
		t.Fatalf("DTO parameter rewrite missing:\n%s", paramStruct)
	}
	dtoContent, err := renderDTO(paramStruct)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(dtoContent, "validator.CommaJoined") != 1 || strings.Count(dtoContent, "FeatureFlags validator.CommaJoined") != 1 {
		t.Fatalf("DTO should contain one rewritten parameter type:\n%s", dtoContent)
	}
	// handler 只引用共享的 DTO 参数结构
	content, err := renderHandler(handlerData)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(content, "dto.DemoParam") != 2 {
		t.Fatalf("expected Add and Edit to use the shared dto parameter struct:\n%s", content)
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
	if !strings.Contains(code, "func (s *DemoRepository) NewRow() any") || !strings.Contains(code, "return &model.Demo{}") {
		t.Fatalf("row factory missing from repository template:\n%s", code)
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

func TestApplySpecDefaultTags(t *testing.T) {
	structContent := `type CountryLanguage struct {
	ID     int64  ` + "`gorm:\"column:id;type:bigint unsigned;primaryKey;autoIncrement:true;comment:ID\" json:\"id\"`" + `
	Lan    string ` + "`gorm:\"column:lan;type:varchar(20);not null;uniqueIndex:uk_country_language_lan,priority:1;comment:语言代码\" json:\"lan\"`" + `
	Name   string ` + "`gorm:\"column:name;type:varchar(50);not null;comment:语言名称\" json:\"name\"`" + `
	Status int32  ` + "`gorm:\"column:status;type:tinyint unsigned;not null;default:1;comment:状态:0=禁用,1=启用\" json:\"status\"`" + `
	Weigh  int32  ` + "`gorm:\"column:weigh;type:int;not null;comment:权重\" json:\"weigh\"`" + `
}`
	fields := []crudmodel.Field{
		{Name: "id", PrimaryKey: true},
		{Name: "lan", DefaultType: "EMPTY STRING"},
		{Name: "name", DefaultType: "EMPTY STRING"},
		{Name: "status", DefaultType: "INPUT", Default: "1"},
		{Name: "weigh", Type: "int", DefaultType: "INPUT", Default: "0"},
	}
	got := applySpecDefaultTags(structContent, fields)
	if !strings.Contains(got, "column:lan;default:'';") {
		t.Fatalf("lan missing default:'' :\n%s", got)
	}
	if !strings.Contains(got, "column:name;default:'';") {
		t.Fatalf("name missing default:'' :\n%s", got)
	}
	// status 已有 gen 产出的 default:1，不得重复插入。
	if strings.Count(got, "default:1") != 1 {
		t.Fatalf("status default:1 duplicated:\n%s", got)
	}
	if !strings.Contains(got, "column:weigh;default:0;") {
		t.Fatalf("weigh missing default:0 :\n%s", got)
	}
	// 主键不插入 default。
	if strings.Contains(got, "column:id;default:") {
		t.Fatalf("primary key got default tag:\n%s", got)
	}
}

func TestSpecDefaultGormTag(t *testing.T) {
	cases := []struct {
		field crudmodel.Field
		want  string
	}{
		{crudmodel.Field{Name: "lan", DefaultType: "EMPTY STRING"}, "default:''"},
		{crudmodel.Field{Name: "weigh", Type: "int", DefaultType: "INPUT", Default: "0"}, "default:0"},
		{crudmodel.Field{Name: "status", Type: "tinyint", DefaultType: "INPUT", Default: "1"}, "default:1"},
		{crudmodel.Field{Name: "title", Type: "varchar", DefaultType: "INPUT", Default: "默认"}, "default:'默认'"},
		{crudmodel.Field{Name: "remark", DefaultType: "NULL"}, ""},
		{crudmodel.Field{Name: "note", DefaultType: "NONE"}, ""},
		{crudmodel.Field{Name: "empty", DefaultType: "INPUT", Default: ""}, ""},
	}
	for _, tc := range cases {
		if got := specDefaultGormTag(tc.field); got != tc.want {
			t.Errorf("specDefaultGormTag(%+v) = %q, want %q", tc.field, got, tc.want)
		}
	}
}
