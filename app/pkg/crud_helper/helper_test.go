package crud_helper

import (
	"encoding/json"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGetPk(t *testing.T) {
	fields := getTestFieldData()
	tablePk := getPk(fields)

	assert.Equal(t, tablePk, "id", "主键是:"+tablePk)
}

func TestGetCommnet(t *testing.T) {
	table := getTestTableData()
	comment := getCommnet(table.Comment)

	assert.Equal(t, comment, "测试管理", "备注:"+comment)
}

func TestParseNameData(t *testing.T) {
	module := "admin"
	tableName := "test1"
	table := getTestTableData()
	modelFile, err := ParseNameData(module, tableName, "model", table.ModelFile)
	content, _ := json.MarshalIndent(modelFile, "", " ")
	fmt.Println(string(content))
	fmt.Println(err)

	handlerFile, err := ParseNameData("admin", tableName, "handler", table.ControllerFile)
	content, _ = json.MarshalIndent(handlerFile, "", " ")
	fmt.Println(string(content))
	fmt.Println(err)
}

func TestParseWebDirNameData(t *testing.T) {
	testTable := getTestTableData()
	testViews := ParseWebDirNameData("test1", "views", testTable.WebViewsDir)
	testLang := ParseWebDirNameData("test1", "lang", testTable.WebViewsDir)
	require.Equal(t, "web/src/views/backend/test1", filepath.ToSlash(testViews.Views))
	require.Equal(t, "web/src/lang/backend/en/test1.ts", filepath.ToSlash(testLang.LangFile("en")))

	nestedViews := ParseWebDirNameData("country_language_content", "views", "web/src/views/backend/country/language/content")
	nestedLang := ParseWebDirNameData("country_language_content", "lang", "web/src/views/backend/country/language/content")
	require.Equal(t, "web/src/views/backend/country/language/content", filepath.ToSlash(nestedViews.Views))
	require.Equal(t, "web/src/lang/backend/zh-cn/country/language/content.ts", filepath.ToSlash(nestedLang.LangFile("zh-cn")))

	// 对齐上游 lcfirst:显式路径的驼峰末段必须保留,
	// 使 views 目录与菜单名(OriginalLastName)一致
	camelViews := ParseWebDirNameData("country_language_content", "views", "web/src/views/backend/country/languageContent")
	camelLang := ParseWebDirNameData("country_language_content", "lang", "web/src/views/backend/country/languageContent")
	require.Equal(t, "web/src/views/backend/country/languageContent", filepath.ToSlash(camelViews.Views))
	require.Equal(t, "web/src/lang/backend/zh-cn/country/languageContent.ts", filepath.ToSlash(camelLang.LangFile("zh-cn")))
	require.Equal(t, "country/languageContent", GetMenuName(camelViews))
}

func TestFieldsMap(t *testing.T) {
	fields := getTestFieldData()

	fieldsMap := map[string]string{}
	for _, field := range fields {
		fieldsMap[field.Name] = field.DesignType
	}
	content, _ := json.MarshalIndent(fieldsMap, "", "  ")
	fmt.Println(string(content))
}

func TestAnalyseField(t *testing.T) {
	fields := getTestFieldData()
	for _, field := range fields {
		field = analyseField(field)
		content, _ := json.MarshalIndent(field, "", "  ")
		fmt.Println(string(content))
	}
}

func TestGetDictData(t *testing.T) {
	table := getTestTableData()
	fields := getTestFieldData()
	langEnData := map[string]string{}
	langZhData := map[string]string{}

	quickSearchFieldZhCnTitle := []string{}

	for _, field := range fields {
		field = analyseField(field)

		getDictData(&langEnData, field, "en", "")
		getDictData(&langZhData, field, "zh-cn", "")

		if slices.Contains(table.QuickSearchField, field.Name) {
			if n, ok := langZhData[field.Name]; ok {
				quickSearchFieldZhCnTitle = append(quickSearchFieldZhCnTitle, n)
			} else {
				quickSearchFieldZhCnTitle = append(quickSearchFieldZhCnTitle, field.Name)
			}
		}

	}

	en, _ := json.MarshalIndent(langEnData, "", "  ")
	fmt.Println(string(en))
	zh, _ := json.MarshalIndent(langZhData, "", "  ")
	fmt.Println(string(zh))
	fmt.Println(quickSearchFieldZhCnTitle)
}

// TestGetRemoteSelectUrl 对齐上游:crud 来源时 URL 由控制器推导;Go 扁平
// handler 文件需通过 router.go 反查真实路由名(user.go -> user.User)。
func TestGetRemoteSelectUrl(t *testing.T) {
	cases := []struct {
		name  string
		field model.Field
		want  string
	}{
		{"user controller resolves registered route", model.Field{Form: model.FormAttr{RemoteController: "app/admin/handler/user.go", RemoteSourceConfigType: "crud"}}, "/admin/user.User/index"},
		{"nested-style controller resolves registered route", model.Field{Form: model.FormAttr{RemoteController: "app/admin/handler/admin_group.go", RemoteSourceConfigType: "crud"}}, "/admin/auth.Group/index"},
		{"backslash path", model.Field{Form: model.FormAttr{RemoteController: `app\admin\handler\user.go`, RemoteSourceConfigType: "crud"}}, "/admin/user.User/index"},
		{"manual url wins for custom source", model.Field{Form: model.FormAttr{RemoteController: "app/admin/handler/user.go", RemoteUrl: "/admin/custom/index", RemoteSourceConfigType: "custom"}}, "/admin/custom/index"},
		{"unknown controller falls back to path derivation", model.Field{Form: model.FormAttr{RemoteController: "app/admin/handler/no_such_handler.go", RemoteSourceConfigType: "crud"}}, "/admin/no_such_handler/index"},
		{"empty controller uses remote url", model.Field{Form: model.FormAttr{RemoteUrl: "/admin/foo/index", RemoteSourceConfigType: "crud"}}, "/admin/foo/index"},
	}
	for _, c := range cases {
		if got := GetRemoteSelectUrl(c.field); got != c.want {
			t.Errorf("%s: GetRemoteSelectUrl() = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestCheckJoinModelKeepsExistingRemoteModel verifies that a remote-model path
// pointing at an existing file (e.g. app/common/model/user.go) is used as-is
// instead of being re-derived under app/admin/model/common/model/ and rebuilt.
func TestCheckJoinModelKeepsExistingRemoteModel(t *testing.T) {
	existing := filepath.Join(utils.RootPath(), "app", "common", "model", "tmp_joinmodel_check_test.go")
	if err := os.WriteFile(existing, []byte("package model\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(existing) })

	field := model.Field{
		Name:       "user_ids",
		DesignType: "remoteSelects",
		Form: model.FormAttr{
			RemoteTable: "user",
			RemoteModel: "app/common/model/tmp_joinmodel_check_test.go",
		},
	}
	rootFileName, err := checkJoinMoel(nil, nil, field, "user", "ba_user")
	if err != nil {
		t.Fatal(err)
	}
	if rootFileName != "" {
		t.Fatalf("existing remote model must not be rebuilt, got rootFileName %q", rootFileName)
	}
	mangled := filepath.Join(utils.RootPath(), "app", "admin", "model", "common", "model", "tmp_joinmodel_check_test.go")
	if _, err := os.Stat(mangled); !os.IsNotExist(err) {
		t.Fatalf("remote model was rebuilt under the wrong admin path: %s", mangled)
	}
}

// TestGenerate_UsesTableDataScope verifies that GenerateFile reads the data-scope
// configuration from the Table value and that temp-dir rendering succeeds.
func TestGenerate_UsesTableDataScope(t *testing.T) {
	table := getTestTableData()
	table.DataScope = &data_scope.Config{Mode: data_scope.ModeNone}
	fields := getCompileFields("")

	getTableName := func(tableName string, fullName bool) string {
		prefix := ""
		if fullName {
			prefix = "ba_"
		}
		tableName = strings.TrimPrefix(tableName, prefix)
		return prefix + tableName
	}

	modelData, handlerData, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, table.DataScope, getTableName, proveAll)
	require.NoError(t, err)
	require.Equal(t, data_scope.ModeNone, modelData.DataScopePolicy.Mode)

	className := modelData.ClassName
	structContent := compileDemoStruct(className, "", "", "")
	modelData.Pk = "id"
	modelData.StructTemp = structContent

	modelCode, err := renderModel(modelData)
	require.NoError(t, err)
	handlerCode, err := renderHandler(handlerData, structContent)
	require.NoError(t, err)
	require.NoError(t, compileDataScopeFixture(t, className, modelCode, handlerCode))
}

func TestModelQuickSearchFieldRendering(t *testing.T) {
	modelData := ModelData{
		Namespace:        "model",
		ClassName:        "CountryLanguageContent",
		Name:             "country_language_content",
		Pk:               "id",
		QuickSearchField: "group,key",
		StructTemp:       "type CountryLanguageContent struct{}",
		DataScopePolicy:  data_scope.ResourcePolicy{Mode: data_scope.ModeNone},
	}

	content, err := renderRawModel(modelData)
	require.NoError(t, err)
	require.Contains(t, content, `QuickSearchField: "group,key"`)
	require.NotContains(t, content, `QuickSearchField: "name"`)

	table := getTestTableData()
	table.QuickSearchField = nil
	fields := getCompileFields("")
	getTableName := func(tableName string, fullName bool) string {
		if fullName {
			return "ba_" + tableName
		}
		return tableName
	}
	prepared, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, table.DataScope, getTableName, proveAll)
	require.NoError(t, err)
	require.Equal(t, "id", prepared.QuickSearchField)
}

func TestGetQuote(t *testing.T) {
	data := "sort"
	content := getQuote(data)
	fmt.Println(content)
}

func TestBuildSimpleArray(t *testing.T) {
	data := []string{"sort", "id", "book"}
	content := buildSimpleArray(data)
	fmt.Println(content)
}

func TestHandleTableDesign(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BUILDADMIN_TEST_MYSQL_DSN not set; skipping DB mutation test")
	}
	table := getTestTableData()
	fields := getTestFieldData()
	fullTableName := "ba_test1"

	db, err := gorm.Open(mysql.Open(dsn))
	require.NoError(t, err)
	HandleTableDesign(db, fullTableName, table, fields)
}

func TestInferDesignTypeHelperRuleMatrix(t *testing.T) {
	cases := []struct {
		name, typ, dataType, want string
		auto                      bool
	}{
		{"id", "bigint", "", "pk", true}, {"weigh", "int", "", "weigh", false},
		{"create_time", "int", "", "timestamp", false}, {"enabled_switch", "tinyint", "", "switch", false},
		{"body_content", "text", "", "editor", false}, {"description_textarea", "varchar", "", "textarea", false},
		{"tags_array", "varchar", "", "array", false}, {"start_datetime", "int", "", "timestamp", false},
		{"published_at", "datetime", "", "datetime", false}, {"birthday", "date", "", "date", false},
		{"birth_year", "year", "", "year", false}, {"alarm_time", "time", "", "time", false},
		{"kind_select", "varchar", "", "select", false}, {"kind_selects", "varchar", "", "selects", false},
		{"user_ids", "varchar", "", "remoteSelects", false}, {"user_id", "bigint", "", "remoteSelect", false},
		{"province_city", "varchar", "", "city", false}, {"cover_image", "varchar", "", "image", false},
		{"cover_images", "varchar", "", "images", false}, {"download_file", "varchar", "", "file", false},
		{"download_files", "varchar", "", "files", false}, {"status", "tinyint", "tinyint(1)", "radio", false},
		{"menu_icon", "varchar", "", "icon", false},
		{"sort_number", "int", "", "number", false}, {"amount", "decimal", "", "number", false},
		{"notes", "text", "", "textarea", false}, {"options", "set", "", "checkbox", false},
		{"theme_color", "varchar", "", "color", false}, {"plain_name", "varchar", "", "string", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := inferDesignTypeForField(model.Field{Name: tc.name, Type: tc.typ, DataType: tc.dataType, AutoIncrement: tc.auto})
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDesignTypeDefaultMatrix(t *testing.T) {
	cases := map[string]struct {
		render, operator, sortable, search string
		width                              int
	}{
		"pk": {"", "RANGE", "custom", "", 70}, "spk": {"", "RANGE", "custom", "", 180}, "weigh": {"", "RANGE", "custom", "", 0},
		"switch": {"switch", "eq", "false", "", 0}, "select": {"tag", "eq", "false", "", 0}, "selects": {"tags", "FIND_IN_SET", "false", "", 0},
		"radio": {"tag", "eq", "false", "", 0}, "checkbox": {"tags", "FIND_IN_SET", "false", "", 0}, "remoteSelect": {"tags", "LIKE", "", "string", 0}, "remoteSelects": {"tags", "FIND_IN_SET", "", "remoteSelect", 0},
		"string": {"none", "LIKE", "false", "", 0}, "textarea": {"", "false", "", "", 0}, "editor": {"", "false", "", "", 0}, "number": {"none", "RANGE", "false", "", 0}, "float": {"none", "RANGE", "false", "", 0},
		"datetime": {"", "RANGE", "custom", "datetime", 160}, "timestamp": {"datetime", "RANGE", "custom", "datetime", 160}, "date": {"", "RANGE", "custom", "date", 0}, "time": {"", "RANGE", "custom", "time", 0}, "year": {"", "RANGE", "custom", "", 0},
		"image": {"image", "false", "", "", 0}, "images": {"images", "false", "", "", 0}, "file": {"none", "false", "", "", 0}, "files": {"none", "false", "", "", 0}, "password": {"", "false", "", "", 0}, "array": {"", "false", "", "", 0}, "city": {"", "false", "", "", 0}, "icon": {"icon", "false", "", "", 0}, "color": {"color", "false", "", "", 0},
	}
	for designType, want := range cases {
		field := model.Field{DesignType: designType}
		applyDesignTypeDefaults(&field)
		if field.Table.Render != want.render || field.Table.Operator != want.operator || field.Table.Sortable != want.sortable || field.Table.ComSearchRender != want.search || field.Table.Width != want.width {
			t.Errorf("%s defaults = %+v", designType, field.Table)
		}
	}
}

func TestDesignTypeFormDefaultMatrix(t *testing.T) {
	tests := map[string]model.FormAttr{
		"password":      {Validator: []string{"password"}},
		"number":        {Validator: []string{"number"}, Step: 1},
		"float":         {Validator: []string{"float"}, Step: 1},
		"textarea":      {Rows: 3},
		"datetime":      {Validator: []string{"date"}},
		"timestamp":     {Validator: []string{"date"}},
		"date":          {Validator: []string{"date"}},
		"year":          {Validator: []string{"date"}},
		"time":          {},
		"selects":       {SelectMulti: "1"},
		"remoteSelect":  {RemotePk: "id", RemoteField: "name"},
		"remoteSelects": {SelectMulti: "1", RemotePk: "id", RemoteField: "name"},
		"editor":        {Validator: []string{"editorRequired"}},
		"images":        {ImageMulti: "1"},
		"files":         {FileMulti: "1"},
	}
	for designType, want := range tests {
		t.Run(designType, func(t *testing.T) {
			field := model.Field{DesignType: designType}
			applyDesignTypeDefaults(&field)
			require.Equal(t, want, field.Form)
		})
	}
}

func TestFractionalStepSurvivesPopupFormRendering(t *testing.T) {
	field := model.Field{Name: "ratio", DesignType: "number", Form: model.FormAttr{Step: 0.001}}
	markup := getFormField(field, nil, "", func(string, bool) string { return "" })
	content, err := renderFormFile(FormVueData{FormFields: []string{markup}}, []model.Field{field}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `:input-attr="{ step: 0.001 }"`) {
		t.Fatalf("fractional step missing from popup form: %s", content)
	}
}

func TestNumberStepZeroKeepsExistingDefault(t *testing.T) {
	field := model.Field{Name: "ratio", DesignType: "number", Form: model.FormAttr{Step: 0}}
	markup := getFormField(field, nil, "", func(string, bool) string { return "" })
	if !strings.Contains(markup, `:input-attr="{ step: 1 }"`) {
		t.Fatalf("zero step default changed: %s", markup)
	}
}

func TestNumberStepNegativePreservesExistingEmission(t *testing.T) {
	field := model.Field{Name: "ratio", DesignType: "number", Form: model.FormAttr{Step: -0.5}}
	markup := getFormField(field, nil, "", func(string, bool) string { return "" })
	if !strings.Contains(markup, `:input-attr="{ step: -0.5 }"`) {
		t.Fatalf("negative step emission changed: %s", markup)
	}
}

func TestGetRemotePk(t *testing.T) {
	for _, tc := range []struct {
		field model.Field
		want  string
	}{
		{model.Field{}, "ba_user.id"},
		{model.Field{Form: model.FormAttr{RemotePk: "uuid"}}, "ba_user.uuid"},
		{model.Field{Form: model.FormAttr{RemotePk: "owner.uuid"}}, "owner.uuid"},
		{model.Field{Form: model.FormAttr{RemotePrimaryTableAlias: "owner", RemotePk: "uuid"}}, "owner.uuid"},
	} {
		if got := GetRemotePk("ba_user", tc.field); got != tc.want {
			t.Fatalf("GetRemotePk()=%q want %q", got, tc.want)
		}
	}
}

func TestGetJsonFromAnyEscapesStringsAndSortsNestedValues(t *testing.T) {
	quotes := "both ' and \" plus \\\n+"
	key := "key'\"\\path"
	got := getJsonFromAny(map[string]any{
		"z": "true", "a": "false", "null": "null", "number": "123", "array": "[1,2]",
		"translate": `t('x')`, "quotes": quotes, "nested": map[string]any{"n": 2}, "numeric": 3.5, key: "value",
	})
	want := `{ a: "false", array: "[1,2]", ` + strconv.Quote(key) + `: "value", nested: { n: 2 }, null: "null", number: "123", numeric: 3.5, quotes: ` + strconv.Quote(quotes) + `, translate: "t('x')", z: "true" }`
	if got != want {
		t.Fatalf("getJsonFromAny() = %q, want %q", got, want)
	}
}

func TestGetTableColumnIncludesSearchInputAttrs(t *testing.T) {
	column := getTableColumn(model.Field{Name: "title", Table: model.TableAttr{ComSearchInputAttr: model.ComSearchInputAttrs{"size": "large"}}}, nil, "", "", "")
	if !strings.Contains(column, `comSearchInputAttr: { size: "large" }`) {
		t.Fatalf("column = %s", column)
	}
}

func TestGeneratedSwitchPartialEditAllowlistIncludesEverySwitch(t *testing.T) {
	fields := []model.Field{{Name: "status", DesignType: "switch"}, {Name: "enabled", DesignType: "switch"}, {Name: "title", DesignType: "string"}}
	handler := HandlerData{Namespace: "handler", ClassName: "Orders", ModelImportPath: "go-build-admin/app/admin/model", ModelName: "Orders", ModelVar: "orders", PkGoType: "int32", PkJSONName: "id", PartialEditFields: buildPartialEditFields(fields)}
	content, err := renderHandler(handler, "type Orders struct {\n\tID int `json:\"id\"`\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `map[string]bool{"status": true, "enabled": true}`) {
		t.Fatalf("switch allowlist missing field: %s", content)
	}
	if strings.Contains(content, `map[string]bool{"status": true}`) {
		t.Fatalf("switch allowlist remained status-only: %s", content)
	}
}

func TestRemoteCommonSearchMetadataIsGeneratedOnce(t *testing.T) {
	field := model.Field{Name: "owner_id", DesignType: "remoteSelect", Form: model.FormAttr{RemoteTable: "owner", RemotePk: "uuid", RemoteField: "name", RemoteUrl: "/admin/owner/options"}}
	metadata := buildRemoteSearchMetadata(field, func(name string, full bool) string { return "ba_" + name })
	column := getTableColumn(model.Field{Name: field.Name, Table: model.TableAttr{ComSearchRender: "remoteSelect", Remote: metadata}}, nil, "", "", "")
	if !strings.Contains(column, `comSearchRender: "remoteSelect"`) || !strings.Contains(column, "remote: {") || strings.Count(column, "comSearchRender:") != 1 {
		t.Fatalf("remote search column metadata incomplete or duplicated: %s", column)
	}
}

func TestBuildEditableColumnsKeepsTableExcludedFormField(t *testing.T) {
	fields := []model.Field{{Name: "title", Form: model.FormAttr{}}, {Name: "hidden_column", TableBuildExclude: true}, {Name: "hidden_form", FormBuildExclude: true}}
	got := buildEditableColumns("id", "", []string{"title", "hidden_column", "hidden_form"}, fields)
	if !slices.Equal(got, []string{"title", "hidden_column"}) {
		t.Fatalf("editable columns = %v", got)
	}
}

func TestCityModelStructIncludesTextAccessor(t *testing.T) {
	structContent := addCityTextFields("type Orders struct {\n\tRegionCity string `json:\"region_city\"`\n}\n", []string{"region_city"})
	if !strings.Contains(structContent, "RegionCityText string `json:\"region_city_text\" gorm:\"-\"`") {
		t.Fatalf("city text accessor missing from struct: %s", structContent)
	}
	modelContent, err := renderModel(ModelData{Namespace: "model", ClassName: "Orders", ModelVar: "orders", Pk: "id", PkGoField: "ID", PkGoType: "int32", StructTemp: structContent, DataScopePolicy: data_scope.ResourcePolicy{Mode: data_scope.ModeNone}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(modelContent, "RegionCityText string `json:\"region_city_text\" gorm:\"-\"`") {
		t.Fatalf("city text accessor missing from rendered model: %s", modelContent)
	}
}

func TestPrepareGenerationDataCarriesCityTextAccessorIntoModelOutput(t *testing.T) {
	table := model.Table{Name: "orders", FormFields: []string{"region_city"}, ColumnFields: []string{"id", "region_city"}, DataScope: &data_scope.Config{Mode: data_scope.ModeNone}}
	fields := []model.Field{
		{Name: "id", Type: "int", PrimaryKey: true, DesignType: "pk"},
		{Name: "region_city", Type: "varchar", DesignType: "city"},
	}
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}
	modelData, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, table.DataScope, getTableName, proveAll)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(modelData.CityTextFields, []string{"region_city"}) {
		t.Fatalf("city metadata = %v", modelData.CityTextFields)
	}
	modelData.StructTemp = addCityTextFields("type Orders struct {\n\tRegionCity string `json:\"region_city\"`\n}\n", modelData.CityTextFields)
	modelContent, err := renderModel(modelData)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(modelContent, "RegionCityText string `json:\"region_city_text\" gorm:\"-\"`") {
		t.Fatalf("production model output lacks city accessor: %s", modelContent)
	}
}

func TestRelationNameForField(t *testing.T) {
	cases := map[string]string{
		"user_id_id": "userId",
		"paid":       "paidTable",
		"user_ids":   "user",
		"user_id":    "user",
	}
	for fieldName, want := range cases {
		if got := relationNameForField(fieldName); got != want {
			t.Errorf("relationNameForField(%q) = %q, want %q", fieldName, got, want)
		}
	}
}

func TestParseWebDirNameDataAutomaticTailAndExplicitUnderscores(t *testing.T) {
	auto := ParseWebDirNameData("country_language_content", "views", "")
	if got := GetMenuName(auto); got != "country/languageContent" {
		t.Fatalf("automatic menu name=%q", got)
	}
	explicit := ParseWebDirNameData("ignored", "views", `web/src/views/backend/some_special_dir/orders`)
	if got := GetMenuName(explicit); got != "some_special_dir/orders" {
		t.Fatalf("explicit menu name=%q", got)
	}
}

func TestParseWebDirNameDataSeparatorsAndInvalidZeroValue(t *testing.T) {
	forward := ParseWebDirNameData("orders", "views", "web/src/views/backend/ops/orders")
	backslash := ParseWebDirNameData("orders", "views", `web\src\views\backend\ops\orders`)
	if GetMenuName(forward) != GetMenuName(backslash) || GetMenuName(forward) != "ops/orders" {
		t.Fatalf("separator menus differ: %q vs %q", GetMenuName(forward), GetMenuName(backslash))
	}
	if got := ParseWebDirNameData("orders", "views", "../escape"); got.Views != "" || got.LastName != "" {
		t.Fatalf("invalid path did not return zero value: %+v", got)
	}
}

func TestParseNameDataPreservesExplicitCamelCaseTail(t *testing.T) {
	for _, input := range []string{"app/admin/model/country/languageContent.go", "app/admin/model/country/language_content.go"} {
		info, err := ParseNameData("admin", "ignored", "model", input)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(info.ParseFile) != filepath.Base(input) {
			t.Fatalf("ParseFile=%q for %q", info.ParseFile, input)
		}
	}
	camel, err := ParseNameData("admin", "ignored", "model", "app/admin/model/country/languageContent.go")
	if err != nil || camel.LastName != "LanguageContent" {
		t.Fatalf("camel explicit info=%+v err=%v", camel, err)
	}
}
