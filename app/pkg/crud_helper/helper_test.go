package crud_helper

import (
	"encoding/json"
	"fmt"
	"go-build-admin/app/pkg/data_scope"
	"os"
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
	tableName := "test1"
	table := getTestTableData()
	webFile := ParseWebDirNameData(tableName, "views", table.WebViewsDir)
	content, _ := json.MarshalIndent(webFile, "", " ")
	fmt.Println(string(content))

	webLangDir := ParseWebDirNameData(tableName, "lang", table.WebViewsDir)
	content, _ = json.MarshalIndent(webLangDir, "", "  ")
	fmt.Println(string(content))

	fmt.Println(strings.Join(webLangDir.Lang, ".") + ".")
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
