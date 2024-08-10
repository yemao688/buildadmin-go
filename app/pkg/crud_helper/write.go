package crud_helper

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func writeModelFile(tablePk string, fieldsMap map[string]string, modelData ModelData, modelFile NameInfo) error {
	data := map[string]string{}
	modelData.Pk = ""
	if tablePk != "id" {
		modelData.Pk = "\n" + Tab(1) + "// 表主键\n" + Tab(1) + "protected $pk =\"" + tablePk + "\"\n" + Tab(1)
	}

	modelData.AutoWriteTimestamp = "false"
	_, ok1 := fieldsMap[createTimeField]
	_, ok2 := fieldsMap[updateTimeField]
	if ok1 || ok2 {
		modelData.AutoWriteTimestamp = "true"
	}

	if modelData.AutoWriteTimestamp == "true" {
		modelData.CreateTime = ""
		if !ok1 {
			modelData.CreateTime = "\n" + Tab(1) + "protected createTime = false"
		}
		modelData.UpdateTime = ""
		if !ok2 {
			modelData.CreateTime = "\n" + Tab(1) + "protected updateTime = false"
		}
	}

	//TODO:

	modelFileContent := assembleStub("mixins/model/model", data, false)
	return writeFile(modelFile.ParseFile, modelFileContent)
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
	data := map[string]string{}

	data["modelNamespace"] = handlerData.Namespace
	data["modelName"] = handlerData.ModelName
	data["filterRule"] = handlerData.FilterRule

	content := assembleStub("mixins/handler", data, false)
	return writeFile(handlerFile.ParseFile, content)
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
