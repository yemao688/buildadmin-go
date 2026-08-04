package crud_helper

import (
	model "buildadmin-go/internal/admin/repository"
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPk(t *testing.T) {
	fields := getTestFieldData()
	tablePk := getPk(fields)

	assert.Equal(t, tablePk, "id", "主键是:"+tablePk)
}

func TestGetComment(t *testing.T) {
	table := getTestTableData()
	comment := getComment(table.Comment)

	assert.Equal(t, comment, "测试管理", "备注:"+comment)
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

	// 视图目录尾段按 SnakeToCamel(..., false) 归一，显式驼峰尾保持不变，
	// 使 views 目录与菜单名(OriginalLastName)一致
	camelViews := ParseWebDirNameData("country_language_content", "views", "web/src/views/backend/country/languageContent")
	camelLang := ParseWebDirNameData("country_language_content", "lang", "web/src/views/backend/country/languageContent")
	require.Equal(t, "web/src/views/backend/country/languageContent", filepath.ToSlash(camelViews.Views))
	require.Equal(t, "web/src/lang/backend/zh-cn/country/languageContent.ts", filepath.ToSlash(camelLang.LangFile("zh-cn")))
	require.Equal(t, "country/languageContent", GetMenuName(camelViews))
}

func TestOptionDictionaryGenerationKeepsPHPCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		field      crudmodel.Field
		wantKeys   []string
		wantLabels []string
	}{
		{
			name:       "radio comment dictionary",
			field:      crudmodel.Field{Name: "visibility", Type: "enum", DataType: "enum('opt0','opt1')", DesignType: "radio", Comment: "单选框:opt0=选项一,opt1=选项二"},
			wantKeys:   []string{"opt0", "opt1"},
			wantLabels: []string{"选项一", "选项二"},
		},
		{
			name:       "select aligned enum",
			field:      crudmodel.Field{Name: "category", Type: "enum", DataType: "enum('tab','link')", DesignType: "select", Comment: "类型:tab=选项卡,link=链接"},
			wantKeys:   []string{"tab", "link"},
			wantLabels: []string{"选项卡", "链接"},
		},
		{
			name:       "checkbox set",
			field:      crudmodel.Field{Name: "features", Type: "set", DataType: "set('feature_a','feature_b')", DesignType: "checkbox", Comment: "功能:feature_a=功能一,feature_b=功能二"},
			wantKeys:   []string{"feature_a", "feature_b"},
			wantLabels: []string{"功能一", "功能二"},
		},
		{
			name:       "selects set",
			field:      crudmodel.Field{Name: "categories", Type: "set", DataType: "set('category_a','category_b')", DesignType: "selects", Comment: "分类:category_a=分类一,category_b=分类二"},
			wantKeys:   []string{"category_a", "category_b"},
			wantLabels: []string{"分类一", "分类二"},
		},
		{
			name:       "label only",
			field:      crudmodel.Field{Name: "plain", Type: "enum", DataType: "enum('a','b')", DesignType: "radio", Comment: "只有标签"},
			wantKeys:   []string{"a", "b"},
			wantLabels: []string{"plain a", "plain b"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := getColumnDict(test.field, "", "")
			labels := map[string]string{}
			getDictData(&labels, test.field, "zh-cn", "")
			for _, key := range test.wantKeys {
				if _, ok := got[key]; !ok {
					t.Fatalf("missing option key %q in %#v", key, got)
				}
			}
			for index, key := range test.wantKeys {
				if len(test.wantLabels) > index && test.name != "label only" && labels[test.field.Name+" "+key] != test.wantLabels[index] {
					t.Errorf("option %q label = %q, want %q", key, labels[test.field.Name+" "+key], test.wantLabels[index])
				}
			}
			if test.name == "label only" && len(got) != len(test.wantKeys) {
				t.Fatalf("label-only comment added orphan dictionary entries: %#v", got)
			}
		})
	}
}

// TestGetRemoteSelectUrl 对齐上游:crud 来源时 URL 由控制器推导;Go 扁平
// handler 文件需通过 router.go 反查真实路由名(user.go -> user.User)。
func TestGetRemoteSelectUrl(t *testing.T) {
	cases := []struct {
		name  string
		field crudmodel.Field
		want  string
	}{
		{"user controller resolves registered route", crudmodel.Field{Form: crudmodel.FormAttr{RemoteController: "internal/admin/handler/user.go", RemoteSourceConfigType: "crud"}}, "/admin/user.User/index"},
		{"nested-style controller resolves registered route", crudmodel.Field{Form: crudmodel.FormAttr{RemoteController: "internal/admin/handler/admin_group.go", RemoteSourceConfigType: "crud"}}, "/admin/auth.Group/index"},
		{"registrar controller resolves route constant", crudmodel.Field{Form: crudmodel.FormAttr{RemoteController: "internal/admin/handler/country_language.go", RemoteSourceConfigType: "crud"}}, "/admin/country.Language/index"},
		{"backslash path", crudmodel.Field{Form: crudmodel.FormAttr{RemoteController: `app\admin\handler\user.go`, RemoteSourceConfigType: "crud"}}, "/admin/user.User/index"},
		{"manual url wins for custom source", crudmodel.Field{Form: crudmodel.FormAttr{RemoteController: "internal/admin/handler/user.go", RemoteUrl: "/admin/custom/index", RemoteSourceConfigType: "custom"}}, "/admin/custom/index"},
		{"unknown controller falls back to path derivation", crudmodel.Field{Form: crudmodel.FormAttr{RemoteController: "internal/admin/handler/no_such_handler.go", RemoteSourceConfigType: "crud"}}, "/admin/no.SuchHandler/index"},
		{"empty controller uses remote url", crudmodel.Field{Form: crudmodel.FormAttr{RemoteUrl: "/admin/foo/index", RemoteSourceConfigType: "crud"}}, "/admin/foo/index"},
	}
	for _, c := range cases {
		if got := GetRemoteSelectUrl(c.field); got != c.want {
			t.Errorf("%s: GetRemoteSelectUrl() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRemoteSelectsEmitsHiddenFKAndVisibleRelationColumns(t *testing.T) {
	field := crudmodel.Field{
		Name:       "reviewer_admins",
		Type:       "varchar",
		DataType:   "varchar(255)",
		DesignType: "remoteSelects",
		Form:       crudmodel.FormAttr{RemoteTable: "admin", RemotePk: "id", RemoteField: "nickname", RelationFields: "nickname,email"},
		Table:      crudmodel.TableAttr{ComSearchRender: "remoteSelect", Operator: "FIND_IN_SET", Remote: `pk: "ba_admin.id", field: "nickname", remoteUrl: "/admin/auth.Admin/index", multiple: true`},
	}
	field = analyseField(field)
	field = prepareGeneratedColumnField(field)
	field.Table.Remote = buildRemoteSearchMetadata(field, func(name string, full bool) string { return "ba_" + name })
	modelData := ModelData{ClassName: "Orders"}
	indexData := IndexVueData{}
	dictEn, dictZh := map[string]string{}, map[string]string{}
	if err := parseJoinData(nil, multiRelationTestColumns(), &dictEn, &dictZh, nil, &modelData, &indexData, field, nil, "reviewer."); err != nil {
		t.Fatal(err)
	}
	if len(modelData.Relations) != 1 || !modelData.Relations[0].Multi {
		t.Fatalf("remoteSelects relation metadata missing: %+v", modelData.Relations)
	}
	columns := strings.Join(indexData.TableColumn, "\n")
	require.Contains(t, columns, `prop: "reviewerAdminsTable.nickname"`)
	require.Contains(t, columns, `prop: "reviewerAdminsTable.email"`)
	require.Contains(t, columns, `operator: false`)
	rawFK := getTableColumn(field, nil, "", "", "")
	require.Contains(t, rawFK, `show: false`)
	require.Contains(t, rawFK, `operator: "FIND_IN_SET"`)
	require.Contains(t, rawFK, `multiple: true`)
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

	modelData, handlerData, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, table.DataScope, getTableName, proveAll)
	require.NoError(t, err)
	require.Equal(t, data_scope.ModeNone, modelData.DataScopePolicy.Mode)

	className := modelData.ClassName
	structContent := compileDemoStruct(className, "", "", "")
	modelData.Pk = "id"
	modelData.StructTemp = structContent

	modelCode, err := renderModel(modelData)
	require.NoError(t, err)
	handlerCode, err := renderHandler(handlerData)
	require.NoError(t, err)
	require.NoError(t, compileDataScopeFixture(t, className, modelCode, handlerCode, modelData.StructTemp))
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
	require.Contains(t, content, `"group,key"`)
	require.NotContains(t, content, `"group,key,name"`)
	require.NotContains(t, content, `"name"`)

	table := getTestTableData()
	table.QuickSearchField = nil
	fields := getCompileFields("")
	getTableName := func(tableName string, fullName bool) string {
		if fullName {
			return "ba_" + tableName
		}
		return tableName
	}
	prepared, _, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, table.DataScope, getTableName, proveAll)
	require.NoError(t, err)
	require.Equal(t, "id", prepared.QuickSearchField)
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
			got := inferDesignTypeForField(crudmodel.Field{Name: tc.name, Type: tc.typ, DataType: tc.dataType, AutoIncrement: tc.auto})
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
		field := crudmodel.Field{DesignType: designType}
		applyDesignTypeDefaults(&field)
		if field.Table.Render != want.render || field.Table.Operator != want.operator || field.Table.Sortable != want.sortable || field.Table.ComSearchRender != want.search || field.Table.Width != want.width {
			t.Errorf("%s defaults = %+v", designType, field.Table)
		}
	}
}

func TestDesignTypeFormDefaultMatrix(t *testing.T) {
	tests := map[string]crudmodel.FormAttr{
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
			field := crudmodel.Field{DesignType: designType}
			applyDesignTypeDefaults(&field)
			require.Equal(t, want, field.Form)
		})
	}
}

func TestFractionalStepSurvivesPopupFormRendering(t *testing.T) {
	field := crudmodel.Field{Name: "ratio", DesignType: "number", Form: crudmodel.FormAttr{Step: 0.001}}
	markup := getFormField(field, nil, "", func(string, bool) string { return "" })
	content, err := renderFormFile(FormVueData{FormFields: []string{markup}}, []crudmodel.Field{field}, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `:input-attr="{ step: 0.001 }"`) {
		t.Fatalf("fractional step missing from popup form: %s", content)
	}
}

func TestNumberStepZeroKeepsExistingDefault(t *testing.T) {
	field := crudmodel.Field{Name: "ratio", DesignType: "number", Form: crudmodel.FormAttr{Step: 0}}
	markup := getFormField(field, nil, "", func(string, bool) string { return "" })
	if !strings.Contains(markup, `:input-attr="{ step: 1 }"`) {
		t.Fatalf("zero step default changed: %s", markup)
	}
}

func TestNumberStepNegativePreservesExistingEmission(t *testing.T) {
	field := crudmodel.Field{Name: "ratio", DesignType: "number", Form: crudmodel.FormAttr{Step: -0.5}}
	markup := getFormField(field, nil, "", func(string, bool) string { return "" })
	if !strings.Contains(markup, `:input-attr="{ step: -0.5 }"`) {
		t.Fatalf("negative step emission changed: %s", markup)
	}
}

func TestGetRemotePk(t *testing.T) {
	for _, tc := range []struct {
		field crudmodel.Field
		want  string
	}{
		{crudmodel.Field{}, "ba_user.id"},
		{crudmodel.Field{Form: crudmodel.FormAttr{RemotePk: "uuid"}}, "ba_user.uuid"},
		{crudmodel.Field{Form: crudmodel.FormAttr{RemotePk: "owner.uuid"}}, "owner.uuid"},
		{crudmodel.Field{Form: crudmodel.FormAttr{RemotePrimaryTableAlias: "owner", RemotePk: "uuid"}}, "owner.uuid"},
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
	column := getTableColumn(crudmodel.Field{Name: "title", Table: crudmodel.TableAttr{ComSearchInputAttr: crudmodel.ComSearchInputAttrs{"size": "large"}}}, nil, "", "", "")
	if !strings.Contains(column, `comSearchInputAttr: { size: "large" }`) {
		t.Fatalf("column = %s", column)
	}
}

func TestGeneratedSwitchPartialEditAllowlistIncludesEverySwitch(t *testing.T) {
	fields := []crudmodel.Field{{Name: "status", DesignType: "switch"}, {Name: "enabled", DesignType: "switch"}, {Name: "title", DesignType: "string"}}
	handler := HandlerData{Namespace: "handler", ClassName: "Orders", ModelImportPath: "buildadmin-go/internal/admin/model", ModelName: "Orders", ModelVar: "orders", PkGoType: "int32", PkJSONName: "id", PartialEditFields: buildPartialEditFields(fields)}
	content, err := renderHandler(handler)
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
	field := crudmodel.Field{Name: "owner_id", DesignType: "remoteSelect", Form: crudmodel.FormAttr{RemoteTable: "owner", RemotePk: "uuid", RemoteField: "name", RemoteUrl: "/admin/owner/options"}}
	metadata := buildRemoteSearchMetadata(field, func(name string, full bool) string { return "ba_" + name })
	column := getTableColumn(crudmodel.Field{Name: field.Name, Table: crudmodel.TableAttr{ComSearchRender: "remoteSelect", Remote: metadata}}, nil, "", "", "")
	if !strings.Contains(column, `comSearchRender: "remoteSelect"`) || !strings.Contains(column, "remote: {") || strings.Count(column, "comSearchRender:") != 1 {
		t.Fatalf("remote search column metadata incomplete or duplicated: %s", column)
	}
}

func TestBuildOperateColumnKeepsFixedRight(t *testing.T) {
	require.Equal(t, ` label: t('Operate'), align: 'center', width: 100, fixed: 'right', render: 'buttons', buttons: optButtons, operator: false`, buildOperateColumn(false))
	require.Equal(t, ` label: t('Operate'), align: 'center', width: 140, fixed: 'right', render: 'buttons', buttons: optButtons, operator: false`, buildOperateColumn(true))
}

func TestRemoteSelectRelationMetadataIsSlimAndRemoteSelectsIsEnriched(t *testing.T) {
	columns := relationTestColumns()
	field := relationTestField("remoteSelect", "username")
	metadata, err := buildRelationMetadata(ParseTableColumns(columns, true), field, "Orders")
	require.NoError(t, err)
	require.Equal(t, "OrdersUserRelation", metadata.DTOName)
	require.Equal(t, []string{"id", "username"}, []string{metadata.Fields[0].JSONName, metadata.Fields[1].JSONName})

	modelData := ModelData{ClassName: "Orders"}
	indexData := IndexVueData{}
	dictEn, dictZh := map[string]string{}, map[string]string{}
	require.NoError(t, parseJoinData(nil, columns, &dictEn, &dictZh, nil, &modelData, &indexData, field, nil, "user."))
	require.Len(t, modelData.Relations, 1)

	deferred := relationTestField("remoteSelects", "username")
	modelData = ModelData{ClassName: "Orders"}
	require.NoError(t, parseJoinData(nil, columns, &dictEn, &dictZh, nil, &modelData, &indexData, deferred, nil, "user."))
	require.Len(t, modelData.Relations, 1)
	require.True(t, modelData.Relations[0].Multi)
}

func TestRemoteSelectRelationRejectsUnknownColumn(t *testing.T) {
	field := relationTestField("remoteSelect", "missing_name")
	_, err := buildRelationMetadata(ParseTableColumns(relationTestColumns(), true), field, "Orders")
	require.ErrorContains(t, err, `unknown relation field "missing_name"`)
}

func TestRemoteSelectsRejectsBadStorageAndRelationCollisions(t *testing.T) {
	badStorage := relationTestField("remoteSelects", "username")
	badStorage.Type = "bigint"
	badStorage.DataType = "bigint"
	_, err := buildRelationMetadata(ParseTableColumns(relationTestColumns(), true), badStorage, "Orders")
	require.ErrorContains(t, err, "string-family CSV storage")

	modelData := ModelData{ClassName: "Orders"}
	indexData := IndexVueData{}
	dictEn, dictZh := map[string]string{}, map[string]string{}
	first := relationTestField("remoteSelects", "username")
	first.Name = "user_ids"
	require.NoError(t, parseJoinData(nil, relationTestColumns(), &dictEn, &dictZh, nil, &modelData, &indexData, first, nil, "user."))
	second := relationTestField("remoteSelect", "username")
	second.Name = "user_id"
	err = parseJoinData(nil, relationTestColumns(), &dictEn, &dictZh, nil, &modelData, &indexData, second, nil, "user.")
	require.ErrorContains(t, err, "relation name/DTO collision")
}

func TestRemoteSelectRelationRenderIncludesSlimLoaderAndCalls(t *testing.T) {
	field := relationTestField("remoteSelect", "username")
	metadata, err := buildRelationMetadata(ParseTableColumns(relationTestColumns(), true), field, "Orders")
	require.NoError(t, err)
	data := ModelData{
		Namespace: "model", ClassName: "Orders", ModelVar: "orders", Pk: "id", PkGoField: "ID", PkGoType: "int32",
		StructTemp: "type Orders struct {\n\tID int32 `gorm:\"column:id\" json:\"id\"`\n}\n",
		Relations:  []RelationMetadata{metadata},
	}
	content, err := renderModel(data)
	require.NoError(t, err)
	_, err = parser.ParseFile(token.NewFileSet(), "orders.go", content, parser.AllErrors)
	require.NoError(t, err)
	_, err = format.Source([]byte(content))
	require.NoError(t, err)
	for _, want := range []string{
		"config *conf.Configuration",
		"func (s *OrdersRepository) loadUserRelations",
		"func (s *OrdersRepository) loadRelations",
		"s.loadRelations(ctx, &list)",
		"s.loadRelations(ctx, &rows)",
		"related := make([]model.OrdersUserRelation, 0)",
	} {
		require.Contains(t, content, want)
	}
	require.NotContains(t, content, "Password")
}

func TestRemoteSelectRelationColumnUsesNestedPropWithoutSearchOperator(t *testing.T) {
	columns := relationTestColumns()
	field := relationTestField("remoteSelect", "username")
	indexData := IndexVueData{}
	dictEn, dictZh := map[string]string{}, map[string]string{}
	modelData := ModelData{ClassName: "Orders"}
	require.NoError(t, parseJoinData(nil, columns, &dictEn, &dictZh, nil, &modelData, &indexData, field, nil, "user."))
	require.Contains(t, strings.Join(indexData.TableColumn, "\n"), `prop: "user.username"`)
	require.Contains(t, strings.Join(indexData.TableColumn, "\n"), `operator: false`)

	field.Table.ComSearchRender = "remoteSelect"
	field.Table.Remote = buildRemoteSearchMetadata(field, func(name string, full bool) string { return "ba_" + name })
	fkColumn := getTableColumn(field, nil, "", "", "")
	require.Contains(t, fkColumn, `prop: "user_id"`)
	require.Contains(t, fkColumn, `comSearchRender: "remoteSelect"`)
}

func TestParseJoinDataSingleRelationFieldUsesLiteralLabel(t *testing.T) {
	field := relationTestField("remoteSelect", "username")
	field.Table.Label = "上级代理"
	indexData := IndexVueData{}
	modelData := ModelData{ClassName: "Orders"}
	dictEn, dictZh := map[string]string{}, map[string]string{}
	require.NoError(t, parseJoinData(nil, relationTestColumns(), &dictEn, &dictZh, nil, &modelData, &indexData, field, nil, "user."))
	require.Contains(t, strings.Join(indexData.TableColumn, "\n"), `label: "上级代理"`)
}

func TestParseJoinDataMultipleRelationFieldsIgnoreSharedLabel(t *testing.T) {
	field := relationTestField("remoteSelects", "nickname,email")
	field.Form.RemoteTable = "admin"
	field.Table.Label = "上级代理"
	indexData := IndexVueData{}
	modelData := ModelData{ClassName: "Orders"}
	dictEn, dictZh := map[string]string{}, map[string]string{}
	require.NoError(t, parseJoinData(nil, multiRelationTestColumns(), &dictEn, &dictZh, nil, &modelData, &indexData, field, nil, "reviewer."))
	columns := strings.Join(indexData.TableColumn, "\n")
	require.NotContains(t, columns, `label: "上级代理"`)
	require.Contains(t, columns, `label: t("reviewer.user__nickname")`)
	require.Contains(t, columns, `label: t("reviewer.user__email")`)
}

func TestParseJoinDataWithoutLabelKeepsDefaultTranslation(t *testing.T) {
	field := relationTestField("remoteSelect", "username")
	indexData := IndexVueData{}
	modelData := ModelData{ClassName: "Orders"}
	dictEn, dictZh := map[string]string{}, map[string]string{}
	require.NoError(t, parseJoinData(nil, relationTestColumns(), &dictEn, &dictZh, nil, &modelData, &indexData, field, nil, "user."))
	require.Contains(t, strings.Join(indexData.TableColumn, "\n"), `label: t("user.user__username")`)
}

func TestOmittedFKColumnStillGeneratesRelationDisplayAndLoader(t *testing.T) {
	table := crudmodel.Table{ColumnFields: []string{"id"}}
	field := relationTestField("remoteSelect", "username")
	field.Table.ComSearchRender = "remoteSelect"
	indexData := IndexVueData{}
	modelData := ModelData{ClassName: "Orders"}
	dictEn, dictZh := map[string]string{}, map[string]string{}
	if slices.Contains(table.ColumnFields, field.Name) {
		indexData.TableColumn = append(indexData.TableColumn, getTableColumn(field, nil, "", "", "user."))
	}
	require.NoError(t, parseJoinData(nil, relationTestColumns(), &dictEn, &dictZh, nil, &modelData, &indexData, field, nil, "user."))
	columns := strings.Join(indexData.TableColumn, "\n")
	require.Contains(t, columns, `prop: "user.username"`)
	require.NotContains(t, columns, `prop: "user_id"`)
	finalizeRelationMetadata(&modelData)
	require.Contains(t, modelData.RelationFields, `User *OrdersUserRelation `)
	require.Contains(t, modelData.RelationLoaders, `loadUserRelations`)
}

func TestRelationFKColumnIsHiddenWithRemoteSearchMetadata(t *testing.T) {
	path := writeSpecTest(t, `name: relation_loader
fields:
  - name: id
    type: bigint
    primaryKey: true
  - name: user_id
    type: bigint
    designType: remoteSelect
    form:
      remoteTable: user
      remotePk: id
      remoteField: username_text
      relationFields: username
`)
	opts, err := LoadSpec(path)
	require.NoError(t, err)
	require.Contains(t, opts.Table.ColumnFields, "user_id")

	field := opts.Fields[1]
	field = prepareGeneratedColumnField(field)
	field.Table.ComSearchRender = "remoteSelect"
	field.Table.Remote = buildRemoteSearchMetadata(field, func(name string, full bool) string { return "ba_" + name })
	fkColumn := getTableColumn(field, nil, "", "", "")
	require.Contains(t, fkColumn, `prop: "user_id"`)
	require.Contains(t, fkColumn, `show: false`)
	require.Contains(t, fkColumn, `comSearchRender: "remoteSelect"`)
	require.NotEmpty(t, field.Table.Remote)

	indexData := IndexVueData{}
	modelData := ModelData{ClassName: "Orders"}
	dictEn, dictZh := map[string]string{}, map[string]string{}
	require.NoError(t, parseJoinData(nil, relationTestColumns(), &dictEn, &dictZh, nil, &modelData, &indexData, field, nil, "user."))
	columns := strings.Join(indexData.TableColumn, "\n")
	require.Contains(t, columns, `prop: "user.username"`)
	require.NotContains(t, columns, `prop: "user_id"`)
	finalizeRelationMetadata(&modelData)
	require.Contains(t, modelData.RelationFields, `User *OrdersUserRelation `)
	require.Contains(t, modelData.RelationLoaders, `loadUserRelations`)

	multi := relationTestField("remoteSelects", "username")
	multi.Form.RemoteTable = "admin"
	multi.Table.ComSearchRender = "remoteSelect"
	multi = prepareGeneratedColumnField(multi)
	multi.Table.Remote = buildRemoteSearchMetadata(multi, func(name string, full bool) string { return "ba_" + name })
	multiColumn := getTableColumn(multi, nil, "", "", "")
	require.Contains(t, multiColumn, `show: false`)
	require.Contains(t, multiColumn, `comSearchRender: "remoteSelect"`)
}

func TestRelationFKExplicitShowTrueIsPreserved(t *testing.T) {
	field := relationTestField("remoteSelect", "username")
	field.Table.Show = "true"
	prepared := prepareGeneratedColumnField(field)
	require.Equal(t, "true", prepared.Table.Show)
}

func TestRemoteSelectsRenderPositionalNullablePayloadLoader(t *testing.T) {
	field := crudmodel.Field{
		Name: "reviewer_admins", Type: "varchar", DataType: "varchar(255)", DesignType: "remoteSelects",
		Form: crudmodel.FormAttr{RemoteTable: "admin", RemotePk: "id", RelationFields: "nickname,email"},
	}
	metadata, err := buildRelationMetadata(ParseTableColumns(multiRelationTestColumns(), true), field, "Orders")
	require.NoError(t, err)
	data := ModelData{
		Namespace: "model", ClassName: "Orders", ModelVar: "orders", Pk: "id", PkGoField: "ID", PkGoType: "int32",
		StructTemp: "type Orders struct {\n\tReviewerAdmins string `gorm:\"column:reviewer_admins\" json:\"reviewer_admins\"`\n}\n",
		Relations:  []RelationMetadata{metadata},
	}
	content, err := renderModel(data)
	require.NoError(t, err)
	_, err = parser.ParseFile(token.NewFileSet(), "orders.go", content, parser.AllErrors)
	require.NoError(t, err)
	_, err = format.Source([]byte(content))
	require.NoError(t, err)
	entityContent, err := renderEntity(data)
	require.NoError(t, err)
	_, err = parser.ParseFile(token.NewFileSet(), "orders.go", entityContent, parser.AllErrors)
	require.NoError(t, err)
	for _, want := range []string{
		"type OrdersReviewerAdminsTableRelationRow struct",
		"type OrdersReviewerAdminsTableRelation struct",
		"Nickname []*string `json:\"nickname\"`",
		"Email",
		"[]*string `json:\"email\"`",
		"ReviewerAdminsTable *OrdersReviewerAdminsTableRelation `gorm:\"-\" json:\"reviewerAdminsTable\"`",
	} {
		require.Contains(t, entityContent, want)
	}
	for _, want := range []string{
		"strings.Split(raw, \",\")",
		"valueIndex int",
		"loadReviewerAdminsTableRelations",
		"loadRelations(ctx, &list)",
		"strconv.ParseInt",
		"if token == \"\"",
	} {
		require.Contains(t, content, want)
	}
}

func relationTestColumns() []model.Column {
	return []model.Column{
		{COLUMN_NAME: "id", DATA_TYPE: "int", COLUMN_TYPE: "int", COLUMN_KEY: "PRI"},
		{COLUMN_NAME: "username", DATA_TYPE: "varchar", COLUMN_TYPE: "varchar(64)"},
		{COLUMN_NAME: "password", DATA_TYPE: "varchar", COLUMN_TYPE: "varchar(128)"},
	}
}

func multiRelationTestColumns() []model.Column {
	return []model.Column{
		{COLUMN_NAME: "id", DATA_TYPE: "int", COLUMN_TYPE: "int", COLUMN_KEY: "PRI"},
		{COLUMN_NAME: "nickname", DATA_TYPE: "varchar", COLUMN_TYPE: "varchar(64)"},
		{COLUMN_NAME: "email", DATA_TYPE: "varchar", COLUMN_TYPE: "varchar(128)"},
	}
}

func relationTestField(designType, relationFields string) crudmodel.Field {
	return crudmodel.Field{
		Name: "user_id", Type: "varchar", DataType: "varchar(255)", DesignType: designType,
		Form: crudmodel.FormAttr{RemoteTable: "user", RemotePk: "id", RemoteField: "username_text", RelationFields: relationFields},
	}
}

func TestBuildEditableColumnsKeepsTableExcludedFormField(t *testing.T) {
	fields := []crudmodel.Field{{Name: "title", Form: crudmodel.FormAttr{}}, {Name: "hidden_column", TableBuildExclude: true}, {Name: "hidden_form", FormBuildExclude: true}}
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
	modelContent, err := renderEntity(ModelData{Namespace: "model", ClassName: "Orders", ModelVar: "orders", Pk: "id", PkGoField: "ID", PkGoType: "int32", StructTemp: structContent, DataScopePolicy: data_scope.ResourcePolicy{Mode: data_scope.ModeNone}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(modelContent, "RegionCityText string `json:\"region_city_text\" gorm:\"-\"`") {
		t.Fatalf("city text accessor missing from rendered entity: %s", modelContent)
	}
}

func TestPrepareGenerationDataCarriesCityTextAccessorIntoModelOutput(t *testing.T) {
	table := crudmodel.Table{Name: "orders", FormFields: []string{"region_city"}, ColumnFields: []string{"id", "region_city"}, DataScope: &data_scope.Config{Mode: data_scope.ModeNone}}
	fields := []crudmodel.Field{
		{Name: "id", Type: "int", PrimaryKey: true, DesignType: "pk"},
		{Name: "region_city", Type: "varchar", DesignType: "city"},
	}
	getTableName := func(name string, full bool) string {
		if full {
			return "ba_" + name
		}
		return name
	}
	modelData, _, _, _, _, _, _, _, _, _, _, _, _, _, err := prepareGenerationData(table, fields, table.DataScope, getTableName, proveAll)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(modelData.CityTextFields, []string{"region_city"}) {
		t.Fatalf("city metadata = %v", modelData.CityTextFields)
	}
	modelData.StructTemp = addCityTextFields("type Orders struct {\n\tRegionCity string `json:\"region_city\"`\n}\n", modelData.CityTextFields)
	modelContent, err := renderEntity(modelData)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(modelContent, "RegionCityText string `json:\"region_city_text\" gorm:\"-\"`") {
		t.Fatalf("production entity output lacks city accessor: %s", modelContent)
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
