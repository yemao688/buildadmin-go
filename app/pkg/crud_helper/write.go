package crud_helper

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func writeWebLangFile(langEnData map[string]string, langZhData map[string]string, webLangDir WebDir) error {
	langTsContent := ""
	for k, v := range langEnData {
		quote := getQuote(v)
		keyStr := formatObjectKey(k)
		langTsContent += Tab(1) + keyStr + ": " + quote + v + quote + ",\n"
	}
	langTsContent = "export default {\n" + langTsContent + "}\n"
	path := utils.RootPath() + webLangDir.En + ".ts"
	return writeFile(path, langTsContent)
}

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

func writeControllerFile(controllerData ControllerData, controllerFile NameInfo) error {
	data := map[string]string{}

	contentFileContent := assembleStub("mixins/controller/controller", data, false)
	return writeFile(controllerFile.ParseFile, contentFileContent)
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

func writeIndexFile(indexVueData IndexVueData, webViewsDir WebDir, controllerFile NameInfo) error {
	data := map[string]string{}

	defaultItems := "{}"
	if len(indexVueData.DefaultItems) > 0 {
		defaultItems = "{" + strings.Join(indexVueData.DefaultItems, ",") + "}"
	}
	data["defaultItems"] = defaultItems
	data["optButtons"] = buildSimpleArray(indexVueData.OptButtons)
	data["tableColumn"] = buildTableColumn(indexVueData.TableColumn)
	data["dblClickNotEditColumn"] = buildSimpleArray(indexVueData.DblClickNotEditColumn)

	controllerFile.Path = append(controllerFile.Path, controllerFile.OriginalLastName)
	data["controllerUrl"] = buildSimpleArray(indexVueData.OptButtons)
	data["componentName"] = buildSimpleArray(indexVueData.OptButtons)

	indexVueContent := assembleStub("html/index", data, false)
	return writeFile(filepath.Join(utils.RootPath(), webViewsDir.Views, "index.vue"), indexVueContent)
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
