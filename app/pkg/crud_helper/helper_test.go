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
