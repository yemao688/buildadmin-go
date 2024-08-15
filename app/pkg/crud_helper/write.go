package crud_helper

import (
	"bytes"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
	"gorm.io/driver/mysql"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func writeModelFile(tablePk string, fullTableName string, tableName string, modelData ModelData, modelFile NameInfo) error {
	if tablePk != "" {
		modelData.Pk = tablePk
	}

	structContent, err := getGenerateStruct(fullTableName, tableName)
	if err != nil {
		return err
	}
	fmt.Println(structContent)
	modelData.StructTemp = structContent

	var buf bytes.Buffer
	tpl, err := template.New(modelTemp).Parse(modelTemp)
	if err != nil {
		return err
	}
	if err := tpl.Execute(&buf, modelData); err != nil {
		return err
	}
	modelContent, err := imports.Process(modelFile.ParseFile, []byte(buf.String()), nil)
	if err != nil {
		return err
	}
	return writeFile(modelFile.ParseFile, string(modelContent))
}

func getGenerateStruct(fullTableName string, tableName string) (string, error) {
	g := gen.NewGenerator(gen.Config{
		OutPath: "./",
		Mode:    gen.WithoutContext | gen.WithDefaultQuery,
		//if you want the nullable field generation property to be pointer type, set FieldNullable true
		// FieldNullable: true,
		//if you want to assign field which has default value in Create API, set FieldCoverable true, reference: https://gorm.io/docs/create.html#Default-Values
		/* FieldCoverable: true,*/
		// if you want generate field with unsigned integer type, set FieldSignable true
		/* FieldSignable: true,*/
		//if you want to generate index tags from database, set FieldWithIndexTag true
		/* FieldWithIndexTag: true,*/
		//if you want to generate type tags from database, set FieldWithTypeTag true
		/* FieldWithTypeTag: true,*/
		//if you need unit tests for query code, set WithUnitTest true
		// WithUnitTest: true,
	})
	db, _ := gorm.Open(mysql.Open("root:root@(127.0.0.1:3306)/buildadmin?charset=utf8mb4&parseTime=True&loc=Local"))
	g.UseDB(db)
	data := g.GenerateModelAs(fullTableName, utils.SnakeToCamel(tableName, true))

	var buf bytes.Buffer
	tpl, err := template.New(StructTmpl).Parse(StructTmpl)
	if err != nil {
		return "", err
	}
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func buildModelAppend() {

}

func buildFormatSimpleArray(data []string, tab int) string {
	if len(data) == 0 {
		return "[]"
	}
	str := "["
	for _, v := range data {
		_, err := strconv.Atoi(v)
		if v == "undefined" || v == "false" || err != nil {
			str += Tab(tab) + v + ","
		} else {
			quote := getQuote(v)
			str += Tab(tab) + quote + v + quote + ", "
		}
	}
	return str + Tab(tab-1) + "]"
}

func buildModelFieldType() {

}

func writeHandlerFile(handlerData HandlerData, handlerFile NameInfo) error {
	var buf bytes.Buffer
	tpl, err := template.New(handlerTemp).Parse(handlerTemp)
	if err != nil {
		return err
	}
	if err := tpl.Execute(&buf, handlerData); err != nil {
		return err
	}
	handlerContent, err := imports.Process(handlerFile.ParseFile, []byte(buf.String()), nil)
	if err != nil {
		return err
	}
	return writeFile(handlerFile.ParseFile, string(handlerContent))
}

func writeFile(path string, content string) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// 创建目录
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func writeWebLangFile(langEnData map[string]string, lang string, webLangDir WebDir) error {
	langTsContent := ""
	for k, v := range langEnData {
		quote := getQuote(v)
		keyStr := formatObjectKey(k)
		langTsContent += Tab(1) + keyStr + ": " + quote + v + quote + ",\n"
	}
	langTsContent = "export default {\n" + langTsContent + "}\n"
	path := filepath.Join(utils.RootPath(), webLangDir.LangDir, lang, webLangDir.LastName+".ts")
	return writeFile(path, langTsContent)
}

func writeIndexFile(indexVueData IndexVueData, webViewsDir WebDir, handlerFile NameInfo) error {
	data := map[string]string{}
	data["webTranslate"] = indexVueData.WebTranslate
	//组件名称
	componentName := webViewsDir.OriginalLastName
	if len(webViewsDir.Path) > 0 {
		componentName = strings.Join(webViewsDir.Path, "/") + "/" + webViewsDir.OriginalLastName
	}
	data["componentName"] = componentName
	data["optButtons"] = buildSimpleArray(indexVueData.OptButtons)
	data["apiUrl"] = "'/admin/" + handlerFile.LastName + "/'"
	data["tablePk"] = indexVueData.TablePk
	data["tableColumn"] = buildTableColumn(indexVueData.TableColumn)
	data["dblClickNotEditColumn"] = buildSimpleArray(indexVueData.DblClickNotEditColumn)
	data["defaultOrder"] = indexVueData.DefaultOrder
	defaultItems := "{}"
	if len(indexVueData.DefaultItems) > 0 {
		defaultItems = "{" + strings.Join(indexVueData.DefaultItems, ",") + "}"
	}
	data["defaultItems"] = defaultItems
	data["enableDragSort"] = indexVueData.EnableDragSort

	indexVueContent := assembleStub("html/index", data, false)
	return writeFile(filepath.Join(utils.RootPath(), webViewsDir.Views, "index.vue"), indexVueContent)
}

func buildSimpleArray(data []string) string {
	if len(data) == 0 {
		return "[]"
	}

	str := ""
	for _, v := range data {
		_, err := strconv.Atoi(v)
		if v == "undefined" || v == "false" || err == nil {
			str += v + ", "
		} else {
			quote := getQuote(v)
			str += quote + v + quote + ", "
		}
	}
	return "[" + strings.TrimRight(str, ", ") + "]"
}

func buildTableColumn(tableColumnList []string) string {
	columnJson := ""
	for _, column := range tableColumnList {
		columnJson += Tab(3) + "{" + strings.TrimRight(column, ",") + " },\n"
	}
	return strings.TrimRight(columnJson, "\n")
}

func writeFormFile(formVueData FormVueData, webViewsDir WebDir, fields []model.Field, webTranslate string) error {
	fieldHtml := "\n"
	data := map[string]string{}
	if formVueData.BigDialog != "" {
		data["bigDialog"] = "\n" + Tab(2) + "width=\"50%\""
	}

	for _, v := range formVueData.FormFields {
		fieldHtml += v
		fieldHtml += " />\n"
	}
	data["formFields"] = strings.TrimRight(fieldHtml, "\n")

	formValidatorRules := map[string][]string{}
	for _, field := range fields {
		if len(field.Form.Validator) > 0 {
			for _, item := range field.Form.Validator {
				message := ""
				if field.Form.ValidatorMsg != "" {
					message = ", message: '" + field.Form.ValidatorMsg + "'"
				}
				formValidatorRules[field.Name] = append(formValidatorRules[field.Name], "buildValidatorData({ name: '"+item+"', title: t('"+webTranslate+field.Name+"')"+message+" })")
			}
		}
	}
	data["formItemRules"] = buildFormValidatorRules(formValidatorRules)
	formVueContent := assembleStub("html/form", data, false)
	return writeFile(filepath.Join(utils.RootPath(), webViewsDir.Views, "popupForm.vue"), formVueContent)
}

func buildFormValidatorRules(formValidatorRules map[string][]string) string {
	rulesHtml := ""
	for key, formItemRule := range formValidatorRules {
		rulesArrHtml := ""
		for _, v := range formItemRule {
			rulesArrHtml += v + ", "
		}
		rulesHtml += Tab(1) + key + ": [" + strings.TrimRight(rulesArrHtml, ", ") + "],\n"
	}
	if rulesHtml != "" {
		return "\n" + rulesHtml
	}
	return rulesHtml
}
