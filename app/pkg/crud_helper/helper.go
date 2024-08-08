package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// 生成表
func GenerateFile(requestType string, table model.Table, fields []model.Field, tableName, fullTableName string) (WebDir, string, error) {
	//主键
	tablePk := getPk(fields)
	//表注释
	tableComment := getCommnet(table.Comment)

	// 生成文件信息解析
	module := "admin"
	if table.IsCommonModel != 0 {
		module = "common"
	}
	modelFile, err := parseNameData(module, tableName, "model", table.ModelFile)
	if err != nil {
		return WebDir{}, "", err
	}
	controllerFile, err := parseNameData("admin", tableName, "handler", table.ControllerFile)
	if err != nil {
		return WebDir{}, "", err
	}
	// validateFile, err := parseNameData("admin", tableName, "validate", table.ValidateFile)

	webViewsDir := ParseWebDirNameData(tableName, "views", table.WebViewsDir)
	webLangDir := ParseWebDirNameData(tableName, "lang", table.WebViewsDir)

	// 语言翻译前缀
	webTranslate := strings.Join(webLangDir.Lang, ".") + "."

	// 快速搜索字段
	if !slices.Contains(table.QuickSearchField, tablePk) {
		table.QuickSearchField = append(table.QuickSearchField, tablePk)
	}
	quickSearchFieldZhCnTitle := []string{}

	// 模型数据
	modelData := ModelData{}
	modelData.Name = tableName
	modelData.ClassName = modelFile.LastName + "Model"
	modelData.Namespace = modelFile.Namespace
	modelData.Append = []string{}
	modelData.Methods = []string{}
	modelData.FieldType = map[string]string{}
	modelData.BeforeInsertMixins = map[string]string{}
	modelData.RelationMethodList = []string{}

	// 控制器数据
	controllerData := ControllerData{}
	controllerData.ClassName = controllerFile.LastName + "Handler"
	controllerData.Namespace = controllerFile.Namespace
	controllerData.TableComment = tableComment
	controllerData.ModelName = modelData.ClassName
	controllerData.ModelNamespace = modelData.Namespace
	controllerData.Use = []string{}
	controllerData.Attr = map[string]string{}
	controllerData.Methods = []string{}

	// index.vue数据
	indexVueData := IndexVueData{}
	indexVueData.EnableDragSort = false
	indexVueData.DefaultItems = []string{}
	indexVueData.TableColumn = []map[string]string{
		{
			"type":     "selection",
			"align":    "center",
			"operator": "false",
		},
	}

	indexVueData.DblClickNotEditColumn = []string{"undefined"}
	indexVueData.OptButtons = []string{"edit", "delete"}
	indexVueData.DefaultOrder = ""

	// form.vue数据
	formVueData := FormVueData{}
	formVueData.BigDialog = "false"
	formVueData.FormFields = []string{}

	// 语言包数据
	langEnData := map[string]string{}
	langZhData := map[string]string{}

	// 简化的字段数据
	fieldsMap := map[string]string{}
	for _, field := range fields {
		fieldsMap[field.Name] = field.DesignType

		//分析字段
		analyseField(&field)

		langEnData = getDictData(langEnData, field, "en", "")
		langZhData = getDictData(langZhData, field, "zh-cn", "")

		// 快速搜索字段
		if slices.Contains(table.QuickSearchField, field.Name) {
			if n, ok := langZhData[field.Name]; ok {
				quickSearchFieldZhCnTitle = append(quickSearchFieldZhCnTitle, n)
			} else {
				quickSearchFieldZhCnTitle = append(quickSearchFieldZhCnTitle, field.Name)
			}
		}

		// 不允许双击编辑的字段
		if field.DesignType == "switch" {
			indexVueData.DblClickNotEditColumn = append(indexVueData.DblClickNotEditColumn, field.Name)
		}

		// 列字典数据
		columnDict := getColumnDict(field, "", webTranslate)

		// 表单项
		if slices.Contains(table.FormFields, field.Name) {
			formFieldHtml := getFormField(field, columnDict, webTranslate, fullTableName)
			formVueData.FormFields = append(formVueData.FormFields, formFieldHtml)
		}

		// 表格列
		indexVueData.DefaultItems = append(indexVueData.DefaultItems, getFieldDefault(field))
		if slices.Contains(table.ColumnFields, field.Name) {
			indexVueData.TableColumn = append(indexVueData.TableColumn, getTableColumn(field, columnDict, "", "", ""))
		}

		// 关联表数据解析
		if slices.Contains([]string{"remoteSelect", "remoteSelects"}, field.DesignType) {
			parseJoinData(field, &langEnData, &langZhData, fullTableName)
		}

		// 模型方法
		parseModelMethods(field, &modelData)

		// 控制器/模型等文件的一些杂项属性解析
		parseSundryData(&controllerData, &indexVueData, &formVueData, field, table)

		if !slices.Contains(table.FormFields, field.Name) {
			controllerData.Attr["preExcludeFields"] = field.Name
		}
	}

	// 快速搜索提示
	langEnData["quick Search Fields"] = strings.Join(table.QuickSearchField, ",")
	langZhData["quick Search Fields"] = strings.Join(quickSearchFieldZhCnTitle, "、")
	controllerData.Attr["quickSearchField"] = strings.Join(table.QuickSearchField, ",")

	// 开启字段排序
	if _, ok := fieldsMap["weigh"]; ok {
		indexVueData.EnableDragSort = true
		modelData.AfterInsert = assembleStub("mixins/model/afterInsert", map[string]string{
			"field": "weight",
		}, false)
	}

	// 表格的操作列
	width := "100"
	if indexVueData.EnableDragSort {
		width = "140"
	}
	indexVueData.TableColumn = append(indexVueData.TableColumn, map[string]string{
		"label":    "t('Operate')",
		"align":    "center",
		"width":    width,
		"render":   "buttons",
		"buttons":  "optButtons",
		"operator": "false",
	})
	if indexVueData.EnableDragSort {
		indexVueData.OptButtons = append([]string{"weigh-sort"}, indexVueData.OptButtons...)
	}

	// 写入模型代码
	// if err := writeModelFile(tablePk, fieldsMap, modelData, modelFile); err != nil {
	// 	return WebDir{}, "", err
	// }

	// 写入控制器代码
	// if err := writeControllerFile(controllerData, controllerFile); err != nil {
	// 	return WebDir{}, "", err
	// }

	// 写入验证器代码
	// validateContent := assembleStub("mixins/validate/validate", map[string]string{
	// 	"namespace": validateFile.Namespace,
	// 	"className": validateFile.LastName,
	// })
	// writeFile(validateFile.ParseFile, validateContent)

	// 写入语言包代码
	if err := writeWebLangFile(langEnData, langZhData, webLangDir); err != nil {
		return WebDir{}, "", err
	}

	// 写入index.vue代码
	indexVueData.TablePk = tablePk
	indexVueData.WebTranslate = webTranslate
	if err := writeIndexFile(indexVueData, webViewsDir, controllerFile); err != nil {
		return WebDir{}, "", err
	}

	// 写入form.vue代码
	if err := writeFormFile(formVueData, webViewsDir, fields, webTranslate); err != nil {
		return WebDir{}, "", err
	}

	return webViewsDir, tableComment, err
}

// 获取表主键
func getPk(fields []model.Field) string {
	pk := "id"
	for _, v := range fields {
		if v.PrimaryKey == "1" {
			pk = v.Name
			break
		}
	}
	return pk
}

// 获取表注释
func getCommnet(comment string) string {
	tableComment := comment
	if strings.HasSuffix(tableComment, "表") {
		tableComment = strings.TrimRight(tableComment, "表") + "管理"
	}
	return tableComment
}

// 解析文件数据
func parseNameData(module string, tableName string, moduleType string, file string) (NameInfo, error) {
	pathArr := []string{}
	if file != "" {
		file = strings.TrimRight(file, ".go")
		file = strings.ReplaceAll(file, ".", "/")
		file = strings.ReplaceAll(file, "/", "/")
		file = strings.ReplaceAll(file, "\\", "/")

		redundantDir := []string{"app", module, moduleType}
		pathArr = strings.Split(file, "/")
		_, pathArr = TrimPrefix(redundantDir, pathArr)
	} else {
		if _, ok := parseNamePresets[moduleType+"/"+tableName]; ok {
			pathArr = parseNamePresets[moduleType+"/"+tableName]
		} else {
			tableName = strings.ReplaceAll(tableName, ".", "/")
			tableName = strings.ReplaceAll(tableName, "/", "/")
			tableName = strings.ReplaceAll(tableName, "\\", "/")
			pathArr = strings.Split(tableName, "/")
		}
	}

	originalLastName := ""
	lastName := ""
	newPathArr := []string{}
	for k, v := range pathArr {
		if len(pathArr)-1 == k {
			originalLastName = v
			lastName = strings.ToLower(v)
		} else {
			newPathArr = append(newPathArr, strings.ToLower(v))
		}
	}

	// 类名不能为内部关键字
	if slices.Contains(reservedKeywords, lastName) {
		return NameInfo{}, cErr.BadRequest("Unable to use internal variable:" + lastName)
	}

	namespace := moduleType
	if len(newPathArr) > 0 {
		namespace = newPathArr[len(newPathArr)-1]
	}
	parseFile := filepath.Join(utils.RootPath(), "app", module, moduleType, filepath.Join(newPathArr...), lastName+".go")
	rootFileName := filepath.Join("go-build-admin/app", module, moduleType, filepath.Join(newPathArr...))

	info := NameInfo{
		LastName:         utils.SnakeToCamel(lastName, true),
		OriginalLastName: originalLastName,
		Path:             newPathArr,
		Namespace:        namespace,
		ParseFile:        parseFile,
		RootFileName:     rootFileName,
	}
	return info, nil
}

func TrimPrefix(slice1, slice2 []string) ([]string, []string) {
	minLen := len(slice1)
	if len(slice2) < minLen {
		minLen = len(slice2)
	}

	// 寻找第一个不匹配的索引
	var i int
	for ; i < minLen; i++ {
		if slice1[i] != slice2[i] {
			break
		}
	}
	// 返回从不匹配索引开始的切片
	return slice1[i:], slice2[i:]
}

func ParseWebDirNameData(tableName string, moduleType string, file string) WebDir {
	pathArr := []string{}
	if file != "" {
		file = strings.TrimRight(file, ".go")
		file = strings.ReplaceAll(file, ".", "/")
		file = strings.ReplaceAll(file, "/", "/")
		file = strings.ReplaceAll(file, "\\", "/")
		file = strings.ReplaceAll(file, "_", "/")

		redundantDir := []string{"web", "src", "views", "backend"}
		pathArr = strings.Split(file, "/")
		_, pathArr = TrimPrefix(redundantDir, pathArr)

	} else {
		if _, ok := parseWebDirPresets[moduleType+"/"+tableName]; ok {
			pathArr = parseWebDirPresets[moduleType+"/"+tableName]
		} else {
			tableName = strings.ReplaceAll(tableName, ".", "/")
			tableName = strings.ReplaceAll(tableName, "/", "/")
			tableName = strings.ReplaceAll(tableName, "\\", "/")
			tableName = strings.ReplaceAll(tableName, "_", "/")
			pathArr = strings.Split(tableName, "/")
		}
	}

	originalLastName := ""
	lastName := ""
	newPathArr := []string{}
	for k, v := range pathArr {
		if len(pathArr)-1 == k {
			originalLastName = v
			lastName = strings.ToLower(v)
		} else {
			newPathArr = append(newPathArr, strings.ToLower(v))
		}
	}

	webDir := WebDir{
		Path:             newPathArr,
		LastName:         lastName,
		OriginalLastName: originalLastName,
	}

	if moduleType == "views" {
		webDir.Views = filepath.Join("web/src/views/backend", strings.Join(newPathArr, "/"), lastName)
	} else if moduleType == "lang" {
		webDir.Lang = append(webDir.Lang, newPathArr...)
		webDir.Lang = append(webDir.Lang, lastName)
		webDir.En = filepath.Join("web/src/lang/backend/en", strings.Join(newPathArr, "/"), lastName)
		webDir.Zh = filepath.Join("web/src/lang/backend/zh-cn/", strings.Join(newPathArr, "/"), lastName)
	}
	return webDir
}

func analyseField(field *model.Field) {
	field.Type = analyseFieldType(field)
	field.OriginalDesignType = field.DesignType

	//表单项类型转换对照表
	designTypeComparison := map[string]string{
		"pk":        "string",
		"weigh":     "number",
		"timestamp": "datetime",
		"float":     "number",
	}
	if _, ok := designTypeComparison[field.DesignType]; ok {
		field.DesignType = designTypeComparison[field.DesignType]
	}

	// 是否开启了多选
	if field.DesignType == "remoteSelect" && field.Form.SelectMulti != "" {
		field.DesignType = field.DesignType + "s"
	}
	if field.DesignType == "select" && field.Form.SelectMulti != "" {
		field.DesignType = field.DesignType + "s"
	}
	if field.DesignType == "image" && field.Form.ImageMulti != "" {
		field.DesignType = field.DesignType + "s"
	}
	if field.DesignType == "file" && field.Form.FileMulti != "" {
		field.DesignType = field.DesignType + "s"
	}
}

// 分析字段数据类型
func analyseFieldType(field *model.Field) string {
	dataType := field.Type
	if field.DataType != "" {
		dataType = field.DataType
	}

	if strings.Contains(dataType, "(") {
		typeName := strings.Split(dataType, "(")
		return strings.TrimSpace(typeName[0])
	}
	return strings.TrimSpace(dataType)
}

// 分析字段的完整数据类型定义
func analyseFieldDataType(field *model.Field) string {
	if field.DataType != "" {
		return field.DataType
	}

	conciseType := analyseFieldType(field)
	limitType, data := analyseFieldLimit(conciseType, field)

	if limitType == "decimalType" {
		return conciseType + "(" + data[0] + "," + data[1] + ")"
	} else if limitType == "valuesType" {
		return conciseType + "(" + strings.Join(data, ",") + ")"
	}
	return conciseType + "(" + data[0] + ")"

}

// 分析字段limit和精度
func analyseFieldLimit(conciseType string, field *model.Field) (string, []string) {
	decimalType := []string{"decimal", "double", "float"}
	valuesType := []string{"enum", "set"}

	dataTL := dataTypeLimit(field.DataType)
	if slices.Contains(decimalType, conciseType) {
		if len(dataTL) == 1 {
			return "decimalType", []string{dataTL[0], "0"}
		}
		if len(dataTL) == 2 {
			return "decimalType", []string{dataTL[0], dataTL[1]}
		}
		precision := "10"
		if field.Length != "" {
			precision = field.Length
		}
		scale := "0"
		if field.Precision != "" {
			scale = field.Precision
		}
		return "decimalType", []string{precision, scale}
	}

	if slices.Contains(valuesType, conciseType) {
		values := []string{}
		for _, v := range dataTL {
			v = strings.ReplaceAll(v, "\"", "")
			v = strings.ReplaceAll(v, "'", "")
			values = append(values, v)
		}
		return "valuesType", values
	}

	if len(dataTL) > 0 {
		return "limitType", []string{dataTL[0]}
	}

	if field.Length != "" {
		return "limitType", []string{field.Length}
	}
	return "", nil
}

func dataTypeLimit(dataType string) []string {
	content := []string{}

	re := regexp.MustCompile(`\((.*?)\)`)
	matches := re.FindStringSubmatch(dataType)

	// 检查是否有匹配项
	if len(matches) > 1 {
		// 分割匹配到的内容
		group := matches[1]
		group = strings.Trim(group, ",")
		content = strings.Split(group, ",")
	}
	return content

}

// 获取字段字典数据
func getDictData(dict map[string]string, field model.Field, lang string, translationPrefix string) map[string]string {
	if field.Comment == "" {
		return dict
	}
	comment := strings.ReplaceAll(field.Comment, "，", ",")
	comment = strings.ReplaceAll(comment, "：", ":")
	if strings.Contains(comment, ":") && strings.Contains(comment, ",") && strings.Contains(comment, "=") {
		commentArr := strings.Split(comment, ":")
		if lang == "en" {
			dict[translationPrefix+field.Name] = field.Name
		} else {
			dict[translationPrefix+field.Name] = commentArr[0]
		}

		items := strings.Split(commentArr[1], ",")
		for _, v := range items {
			valArr := strings.Split(v, "=")
			if len(valArr) == 2 {
				if lang == "en" {
					dict[translationPrefix+field.Name+" "+valArr[0]] = field.Name + " " + valArr[0]
				} else {
					dict[translationPrefix+field.Name+" "+valArr[0]] = valArr[1]
				}
			}
		}
	} else {
		if lang == "en" {
			dict[translationPrefix+field.Name] = field.Name
		} else {
			dict[translationPrefix+field.Name] = comment
		}
	}
	return dict
}

func getColumnDict(column model.Field, translationPrefix string, webTranslate string) map[string]string {
	dict := map[string]string{}
	// 确保字典中无翻译也可以识别到该值
	if slices.Contains([]string{"enum", "set"}, column.Type) {
		dataType := strings.ReplaceAll(column.DataType, " ", "")
		leftBracketPos := strings.Index(dataType, "(")
		rightBracketPos := strings.LastIndex(dataType, ")")
		content := dataType[leftBracketPos+1 : rightBracketPos]
		content = strings.ReplaceAll(content, "\"", "")
		content = strings.ReplaceAll(content, "'", "")
		columnData := strings.Split(content, ",")
		for _, v := range columnData {
			dict[v] = column.Name + " " + v
		}
	}

	dictData := map[string]string{}
	dictData = getDictData(dictData, column, "zh-cn", translationPrefix)
	if len(dictData) > 0 {
		for k := range dictData {
			if translationPrefix+column.Name != k {
				keyName := strings.ReplaceAll(k, translationPrefix+column.Name+" ", "")
				dict[keyName] = "t('" + webTranslate + k + "')"
			}
		}
	}
	return dict

}

func getFormField(field model.Field, columnDict map[string]string, webTranslate string, fullTableName string) string {

	fieldHtml := Tab(5) + "<FormItem"
	// 表单项属性
	fieldHtml += " :label=\"t('" + webTranslate + field.Name + "')\""
	fieldHtml += " type=\"" + field.DesignType + "\""
	if field.DesignType == "number" {
		fieldHtml += " v-model.number=\"baTable.form.items!." + field.Name + "\""
	} else {
		fieldHtml += " v-model=\"baTable.form.items!." + field.Name + "\""
	}
	fieldHtml += " prop=\"" + field.Name + "\""

	// 不同输入框的属性处理
	if len(columnDict) > 0 || slices.Contains([]string{"radio", "checkbox", "select", "selects"}, field.DesignType) {
		fieldHtml += " :data=\"{ content: " + getJsonFromArray(columnDict) + " }\" "

	} else if field.DesignType == "textarea" {
		rows := 3
		if field.Form.Rows != "0" {
			rows, _ = strconv.Atoi(field.Form.Rows)
		}

		fieldHtml += " :input-attr=\"{ rows: " + strconv.Itoa(rows) + "}\" "
		fieldHtml += " @keyup.enter.stop=\"\" "
		fieldHtml += " @keyup.ctrl.enter=\"baTable.onSubmit(formRef)\" "

	} else if field.DesignType == "remoteSelect" || field.DesignType == "remoteSelects" {
		fName := "name"
		if field.Form.RemoteField != "" {
			fName = field.Form.RemoteField
		}
		attr := map[string]string{
			"pk":         GetRemotePk(fullTableName, field),
			"field":      fName,
			"remote-url": GetRemoteSelectUrl(field),
		}
		fieldHtml += " :input-attr=\"" + getJsonFromArray(attr) + "\" "

	} else if field.DesignType == "number" {
		step := 1
		if field.Form.Step != "0" {
			step, _ = strconv.Atoi(field.Form.Step)
		}
		fieldHtml += " :input-attr=\"{ step:" + strconv.Itoa(step) + "}\" "

	} else if field.DesignType == "icon" {
		fieldHtml += " :input-attr=\"" + getJsonFromArray(map[string]string{"placement": "top"}) + "\""

	} else if field.DesignType == "editor" {
		fieldHtml += " @keyup.enter.stop=\"\""
		fieldHtml += " @keyup.ctrl.enter=\"baTable.onSubmit(formRef)\""
	}

	// placeholder
	if !slices.Contains([]string{"image", "images", "file", "files", "switch"}, field.DesignType) {
		if slices.Contains([]string{"radio", "checkbox", "datetime", "year", "date", "time", "select", "selects", "remoteSelect", "remoteSelects", "city", "icon"}, field.DesignType) {
			fieldHtml += " :placeholder=\"t('Please select field', { field: t('" + webTranslate + field.Name + "') })\""
		} else {
			fieldHtml += " :placeholder=\"t('Please input field', { field: t('" + webTranslate + field.Name + "') })\""
		}
	}
	return fieldHtml
}

func getFieldDefault(field model.Field) string {
	defaultValue := ""
	// 默认值
	if field.Default != "" && field.Default != "empty string" {

		defaultValue = field.Name + ":'" + field.Default + "'"
	}

	if field.Default == "null" {
		defaultValue = field.Name + ": null"
	}

	if field.Default == "0" && slices.Contains([]string{"radio", "checkbox", "select", "selects"}, field.DesignType) {
		defaultValue = field.Name + ": '0'"
	}

	if field.DesignType == "array" {
		defaultValue = field.Name + ": []"
	}

	if slices.Contains(dtStringToArray, field.DesignType) && field.Default != "" && strings.Contains(field.Default, ",") {
		defaultValue = field.Name + ":[" + field.Default + "]"
	}

	if slices.Contains([]string{"weigh", "number", "float"}, field.DesignType) {
		defaultValue = field.Name + ":" + field.Default
	}
	return defaultValue
}

func buildSimpleArray(data []string) string {
	if len(data) == 0 {
		return "[]"
	}

	str := ""
	for _, v := range data {
		_, err := strconv.Atoi(v)
		if v == "undefined" || v == "false" || err != nil {
			str += v + ", "
		} else {
			quote := getQuote(v)
			str += quote + v + quote + ", "
		}
	}
	return "[" + strings.TrimRight(str, ", ") + "]"
}

func GetRemotePk(fullTableName string, field model.Field) string {
	name := fullTableName
	if field.Form.RemotePk != "" {
		return name + ".id"
	}
	return name + "." + field.Form.RemotePk
}

func GetRemoteSelectUrl(field model.Field) string {

	if field.Form.RemoteUrl != "" {
		return field.Form.RemoteUrl
	}

	url := ""
	if field.Form.RemoteController != "" {
		redundantDir := []string{"app", "admin", "controller"}
		pathArr := strings.Split(field.Form.RemoteController, "/")
		_, pathArr = TrimPrefix(redundantDir, pathArr)
		if len(pathArr) == 1 {
			url = pathArr[0]
		}

		if len(pathArr) > 1 {
			url = strings.Join(pathArr, ".")
		}
		url = "/admin/" + url + "/index"
	}
	return url
}

func getTableColumn(field model.Field, columnDict map[string]string, fieldNamePrefix string, translationPrefix string, webTranslate string) map[string]string {
	prop := ""
	if field.DesignType == "city" {
		prop = "_text"
	}

	column := map[string]string{
		"label": "t('" + webTranslate + translationPrefix + field.Name + "')",
		"prop":  fieldNamePrefix + field.Name + prop,
		"align": "center",
	}

	// 模糊搜索增加一个placeholder
	if field.Table.Operator != "" && field.Table.Operator == "LIKE" {
		column["operatorPlaceholder"] = "t('Fuzzy query')"
	}

	// 合并前端预设的字段表格属性
	if field.Table.Width != "" {
		column["width"] = field.Table.Width
	}
	if field.Table.Operator != "" {
		column["operator"] = field.Table.Operator
	}
	if field.Table.Sortable != "" {
		column["sortable"] = field.Table.Sortable
	}
	if field.Table.Render != "" {
		column["render"] = field.Table.Render
	}
	if field.Table.TimeFormat != "" {
		column["timeFormat"] = field.Table.TimeFormat
	}

	// 需要值替换的渲染类型
	columnReplaceValue := []string{"tag", "tags", "switch"}
	if !slices.Contains([]string{"remoteSelect", "remoteSelects"}, field.DesignType) && (len(columnDict) > 0 || slices.Contains(columnReplaceValue, field.Table.Render)) {
		itemJson := ""
		for k, v := range columnDict {
			itemJson += buildTableColumnKey(k, v)
		}
		column["replaceValue"] = buildTableColumnKey("replaceValue", itemJson)
	}

	if field.Table.Render == "none" {
		delete(column, "render")
	}
	return column
}

// 关联表数据解析
func parseJoinData(field model.Field, langEnData, langZhData *map[string]string, fullTableName string) {
	// dictEn := map[string]string{}
	// dictZhCn := map[string]string{}

	// if field.Form.RelationFields != "" && field.Form.RemoteTable != "" {
	// 	relationFields := strings.Split(field.Form.RelationFields, ",")
	// 	tableName :=
	// }

}

// 解析模型方法（设置器、获取器等）
func parseModelMethods(field model.Field, modelData *ModelData) {
	// fieldType
	if field.DesignType == "array" {
		modelData.FieldType[field.Name] = "json"
	} else if !slices.Contains([]string{"create_time", "update_time", "updatetime", "createtime"}, field.Name) && field.DesignType == "datetime" &&
		slices.Contains([]string{"int", "bigint"}, field.Type) {
		modelData.FieldType[field.Name] = "timestamp:Y-m-d H:i:s"
	}

	// beforeInsertMixins
	if field.DesignType == "spk" {
		modelData.BeforeInsertMixins["snowflake"] = assembleStub("mixins/model/mixins/beforeInsertWithSnowflake", map[string]string{}, false)
	}

	// methods
	fieldName := utils.SnakeToCamel(field.Name, true)
	if slices.Contains(dtStringToArray, field.DesignType) {
		modelData.Methods = append(modelData.Methods, assembleStub("mixins/model/getters/stringToArray", map[string]string{
			"field": fieldName,
		}, false))
	} else if field.DesignType == "array" {
		modelData.Methods = append(modelData.Methods, assembleStub("mixins/model/getters/jsonDecode", map[string]string{
			"field": fieldName,
		}, false))
	} else if field.DesignType == "time" {
		modelData.Methods = append(modelData.Methods, assembleStub("mixins/model/setters/time", map[string]string{
			"field": fieldName,
		}, false))
	} else if field.DesignType == "editor" {
		modelData.Methods = append(modelData.Methods, assembleStub("mixins/model/getters/htmlDecode", map[string]string{
			"field": fieldName,
		}, false))
	} else if field.DesignType == "spk" {
		modelData.Methods = append(modelData.Methods, assembleStub("mixins/model/getters/string", map[string]string{
			"field": fieldName,
		}, false))
	} else if field.DesignType == "float" {
		modelData.Methods = append(modelData.Methods, assembleStub("mixins/model/getters/float", map[string]string{
			"field": fieldName,
		}, false))
	} else if field.DesignType == "city" {
		modelData.Append = append(modelData.Append, field.Name+"_text")
		modelData.Methods = append(modelData.Methods, assembleStub("mixins/model/getters/cityNames", map[string]string{
			"field":             fieldName + "Text",
			"originalFieldName": field.Name,
		}, false))
	}

}

// 控制器/模型等文件的一些杂项属性解析
func parseSundryData(controllerData *ControllerData, indexVueData *IndexVueData, formVueData *FormVueData, field model.Field, table model.Table) {
	if field.DesignType == "editor" {
		formVueData.BigDialog = "true"
		controllerData.FilterRule = "\n" + Tab(2) + "$this->request->filter('clean_xss')"
	}

	//默认排序字段
	if table.DefaultSortField != "" && table.DefaultSortType != "" {
		defaultSortField := table.DefaultSortField + "," + table.DefaultSortType
		if defaultSortField == "id,desc" {
			controllerData.Attr["defaultSortField"] = ""
		} else {
			controllerData.Attr["defaultSortField"] = defaultSortField
			indexVueData.DefaultOrder = buildDefaultOrder(table.DefaultSortField, table.DefaultSortType)
		}
	}
}

func buildDefaultOrder(field string, t string) string {
	if field != "" && t != "" {
		defaultOrderStub := map[string]string{
			"prop":  field,
			"order": t,
		}
		defaultOrder := getJsonFromArray(defaultOrderStub)
		if defaultOrder != "" {
			return "\n" + Tab(2) + "defaultOrder: " + defaultOrder + ","
		}
	}
	return ""
}

// 获取基础模板文件路径
func getStubFilePath(name string) string {
	return filepath.Join(utils.RootPath(), "app", "pkg", "crud_helper", "stubs", name+".stub")
}

// 组装模板
func assembleStub(name string, data map[string]string, escapeStr bool) string {
	stubPath := getStubFilePath(name)
	content, _ := os.ReadFile(stubPath)
	stubContent := string(content)
	for k, v := range data {
		stubContent = strings.ReplaceAll(stubContent, "{%"+k+"%}", v)
	}

	if escapeStr {
		return escape(stubContent)
	}
	return stubContent

}

// 获取转义编码后的值
func escape(value string) string {
	//TODO: 获取转义编码后的值
	return value
}

func Tab(num int) string {
	return strings.Repeat(" ", 4*num)
}

func buildTableColumn(tableColumnList []map[string]string) string {
	columnJson := ""
	for _, column := range tableColumnList {
		columnJson := Tab(3) + "{"
		for k, v := range column {
			columnJson += buildTableColumnKey(k, v)
		}
		columnJson = strings.TrimRight(columnJson, ",")
		columnJson += " }" + ",\n"
	}
	return strings.TrimRight(columnJson, "\n")
}

func buildTableColumnKey(key string, val string) string {
	itemJson := ""
	key = formatObjectKey(key)
	if val == "false" || val == "true" {
		itemJson = " " + key + ": " + val + ","
	} else if slices.Contains([]string{"label", "width", "buttons"}, key) || strings.HasPrefix(val, "t('") || strings.HasPrefix(val, "t(\"") {
		itemJson = " " + key + ": " + val + ","
	} else {
		itemJson = " " + key + ": '" + val + "',"
	}
	return itemJson
}

func formatObjectKey(keyName string) string {
	re := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]+$`)

	if re.MatchString(keyName) {
		return keyName
	}
	quote := getQuote(keyName)
	return fmt.Sprintf("%s%s%s", quote, keyName, quote)
}

func getQuote(value string) string {
	if !strings.Contains(value, "'") {
		return "'"
	}
	return "\""
}

func getJsonFromArray(data map[string]string) string {
	jsonStr := ""
	for k, v := range data {
		keyStr := " " + formatObjectKey(k) + ": "
		if v == "false" || v == "true" {
			jsonStr += keyStr + v + ","
		} else if v == "null" {
			jsonStr += keyStr + "null,"
		} else if strings.HasPrefix(v, "t('") || strings.HasPrefix(v, "t(\"") || v == "[]" || isNumeric(v) {
			jsonStr += keyStr + v + ","
		} else if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
			jsonStr += keyStr + v + ","
		} else {
			quote := getQuote(v)
			jsonStr += keyStr + quote + v + quote
		}
	}

	if jsonStr == "" {
		return "{}"
	}
	return "{" + strings.TrimRight(jsonStr, ",") + " }"
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
