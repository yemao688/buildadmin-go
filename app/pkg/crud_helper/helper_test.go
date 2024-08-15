package crud_helper

import (
	"encoding/json"
	"fmt"
	"go-build-admin/app/admin/model"
	"slices"
	"strings"
	"testing"

	"github.com/magiconair/properties/assert"
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
	modelFile, err := parseNameData(module, tableName, "model", table.ModelFile)
	content, _ := json.MarshalIndent(modelFile, "", " ")
	fmt.Println(string(content))
	fmt.Println(err)

	handlerFile, err := parseNameData("admin", tableName, "handler", table.ControllerFile)
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

func TestGenerate(t *testing.T) {
	table := getTestTableData()
	fields := getTestFieldData()

	getTableName := func(tableName string, fullName bool) string {
		prefix := ""
		if fullName {
			prefix = "ba_"
		}
		tableName = strings.TrimPrefix(tableName, prefix)
		return prefix + tableName
	}

	db, _ := gorm.Open(mysql.Open("root:root@(127.0.0.1:3306)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local"))
	getColumns := func(tableName string) ([]model.Column, error) {
		result := []model.Column{}
		err := db.Raw("SELECT * FROM `information_schema`.`columns`  WHERE TABLE_SCHEMA = ? AND table_name = ? ORDER BY ORDINAL_POSITION", "buildadmin", getTableName(tableName, true)).Scan(&result).Error
		if err != nil {
			return result, err
		}
		return result, nil
	}
	_, _, err := GenerateFile(table, fields, getTableName, getColumns)
	// fmt.Println(webDir)
	// fmt.Println(lang)
	fmt.Println(err)
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
	table := getTestTableData()
	fields := getTestFieldData()
	fullTableName := "ba_test1"

	db, _ := gorm.Open(mysql.Open("root:root@(127.0.0.1:3306)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local"))
	HandleTableDesign(db, fullTableName, table, fields)
}
