package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// 生成表
func GenerateFile(table model.Table, fields []model.Field, getTableName GetTableName, getColumns GetColumns, db *gorm.DB) (WebDir, string, error) {
	if err := ValidateGenerationInput(table, fields); err != nil {
		return WebDir{}, "", err
	}
	return GenerateFileWithRouteRegistrar(table, fields, table.DataScope, getTableName, getColumns, db, nil)
}

// prepareGenerationData resolves data-scope policy and initializes the model/handler
// data structures used by both production generation and compile-only tests.
func prepareGenerationData(table model.Table, fields []model.Field, dsConfig *data_scope.Config, getTableName GetTableName, proveIndex func(string) (bool, error)) (ModelData, HandlerData, NameInfo, NameInfo, WebDir, WebDir, string, string, string, string, string, error) {
	tableName := getTableName(table.Name, false)
	fullTableName := getTableName(table.Name, true)
	//主键
	tablePk := getPk(fields)
	//表注释
	tableComment := getCommnet(table.Comment)
	// 生成文件信息解析
	module := "admin"
	if table.IsCommonModel != 0 {
		module = "common"
	}
	modelFile, err := ParseNameData(module, tableName, "model", table.ModelFile)
	if err != nil {
		return ModelData{}, HandlerData{}, NameInfo{}, NameInfo{}, WebDir{}, WebDir{}, "", "", "", "", "", err
	}
	handlerFile, err := ParseNameData("admin", tableName, "handler", table.ControllerFile)
	if err != nil {
		return ModelData{}, HandlerData{}, NameInfo{}, NameInfo{}, WebDir{}, WebDir{}, "", "", "", "", "", err
	}

	webViewsDir := ParseWebDirNameData(tableName, "views", table.WebViewsDir)
	webLangDir := ParseWebDirNameData(tableName, "lang", table.WebViewsDir)

	// 语言翻译前缀
	webTranslate := strings.Join(webLangDir.Lang, ".") + "."

	// 快速搜索字段
	if !slices.Contains(table.QuickSearchField, tablePk) {
		table.QuickSearchField = append(table.QuickSearchField, tablePk)
	}

	// 模型数据
	modelData := ModelData{}
	modelData.Namespace = modelFile.Namespace
	modelData.Name = tableName
	modelData.ClassName = modelFile.LastName
	modelData.ModelVar = strings.ToLower(string(modelFile.LastName[0])) + modelFile.LastName[1:]
	modelData.QuickSearchField = strings.Join(table.QuickSearchField, ",")
	pkField := searchField(fields, tablePk)
	modelData.PkGoType, err = primaryKeyGoType(pkField)
	if err != nil {
		return ModelData{}, HandlerData{}, NameInfo{}, NameInfo{}, WebDir{}, WebDir{}, "", "", "", "", "", err
	}

	modelData.Append = []string{}
	modelData.Methods = []string{}
	modelData.FieldType = map[string]string{}
	modelData.BeforeInsertMixins = map[string]string{}
	modelData.RelationMethodList = map[string]string{}

	// 控制器数据
	handlerData := HandlerData{}
	handlerData.Namespace = handlerFile.Namespace
	handlerData.ModelNamespace = modelData.Namespace
	handlerData.ModelImportPath = "go-build-admin/app/" + module + "/" + modelData.Namespace
	handlerData.ClassName = handlerFile.LastName
	handlerData.ModelName = modelData.ClassName
	handlerData.ModelVar = strings.ToLower(string(modelFile.LastName[0])) + modelFile.LastName[1:]
	handlerData.PkGoType = modelData.PkGoType
	handlerData.PkJSONName = tablePk
	handlerData.TableComment = tableComment

	handlerData.Import = []string{}
	handlerData.Attr = map[string]string{}
	handlerData.Methods = []string{}
	handlerData.RelationVisibleFieldList = map[string][]string{}

	// 数据权限解析：只有用户显式持久化 ModeNone 时才允许 admin_id 资源走 none。
	allowNoneExplicit := dsConfig != nil && dsConfig.Mode == data_scope.ModeNone
	ds, err := ResolveDataScope(dsConfig, fields, DataScopeResolveOptions{
		AllowNoneWithAdminID: allowNoneExplicit,
		ProveIndex:           proveIndex,
	})
	if err != nil {
		return ModelData{}, HandlerData{}, NameInfo{}, NameInfo{}, WebDir{}, WebDir{}, "", "", "", "", "", err
	}
	modelData.PkGoField = pkGoField(tablePk)
	modelData.DataScopePolicy = ds.Policy
	modelData.DataScopeOwnerGoField = ds.OwnerGoField
	if ds.OwnerColumn != "" {
		ownerType, err := ownerGoType(searchField(fields, ds.OwnerColumn))
		if err != nil {
			return ModelData{}, HandlerData{}, NameInfo{}, NameInfo{}, WebDir{}, WebDir{}, "", "", "", "", "", err
		}
		modelData.DataScopeOwnerGoType = ownerType
	}
	effectiveFormFields := slices.Clone(table.FormFields)
	if ds.OwnerColumn != "" {
		effectiveFormFields = slices.DeleteFunc(effectiveFormFields, func(s string) bool {
			return s == ds.OwnerColumn
		})
	}
	modelData.EffectiveFormFields = effectiveFormFields
	modelData.EditableColumns = buildEditableColumns(tablePk, ds.OwnerColumn, effectiveFormFields, fields)
	modelData.EditableColumnsGo = joinQuotedColumns(modelData.EditableColumns)

	if ds.OwnerColumn != "" {
		handlerData.ExcludeParamFields = []string{ds.OwnerColumn}
	}

	return modelData, handlerData, modelFile, handlerFile, webViewsDir, webLangDir, webTranslate, tableComment, tablePk, tableName, fullTableName, nil
}

// GenerateFileWithDataScope generates CRUD files using the persisted data-scope
// configuration. A nil dsConfig preserves legacy auto-detection behavior.
func GenerateFileWithDataScope(table model.Table, fields []model.Field, dsConfig *data_scope.Config, getTableName GetTableName, getColumns GetColumns, db *gorm.DB) (WebDir, string, error) {
	return GenerateFileWithRouteRegistrar(table, fields, dsConfig, getTableName, getColumns, db, nil)
}

func GenerateFileWithRouteRegistrar(table model.Table, fields []model.Field, dsConfig *data_scope.Config, getTableName GetTableName, getColumns GetColumns, db *gorm.DB, registrar func(method, path string)) (WebDir, string, error) {
	if err := ValidateGenerationInput(table, fields); err != nil {
		return WebDir{}, "", err
	}
	fullTableName := getTableName(table.Name, true)
	modelData, handlerData, modelFile, handlerFile, webViewsDir, webLangDir, webTranslate, tableComment, tablePk, tableName, fullTableName, err := prepareGenerationData(table, fields, dsConfig, getTableName, buildIndexProver(db, fullTableName))
	if err != nil {
		return WebDir{}, "", err
	}
	handlerData.RegisterAtomicRoute = registrar
	table.FormFields = slices.Clone(modelData.EffectiveFormFields)
	quickSearchFieldZhCnTitle := []string{}

	indexVueData := IndexVueData{}
	indexVueData.EnableDragSort = "false"
	indexVueData.DefaultItems = []string{}
	indexVueData.TableColumn = []string{" type: 'selection', align: 'center', operator: false"}
	indexVueData.DblClickNotEditColumn = []string{"undefined"}
	indexVueData.OptButtons = []string{"edit", "delete"}
	indexVueData.DefaultOrder = ""

	// form.vue数据
	formVueData := FormVueData{}
	formVueData.BigDialog = "false"
	formVueData.FormFields = buildFormFieldMarkup(table.FormFields, fields, webTranslate, getTableName)

	// 语言包数据
	langEnData := map[string]string{}
	langZhData := map[string]string{}

	// 简化的字段数据
	fieldsMap := map[string]string{}
	for _, field := range fields {
		fieldsMap[field.Name] = field.DesignType

		//分析字段
		field = analyseField(field)

		getDictData(&langEnData, field, "en", "")
		getDictData(&langZhData, field, "zh-cn", "")

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
			fieldDefault := getFieldDefault(field)
			if fieldDefault != "" {
				indexVueData.DefaultItems = append(indexVueData.DefaultItems, fieldDefault)
			}
		}

		// 表格列
		if slices.Contains(table.ColumnFields, field.Name) {
			indexVueData.TableColumn = append(indexVueData.TableColumn, getTableColumn(field, columnDict, "", "", webTranslate))
		}

		// 关联表数据解析
		if slices.Contains([]string{"remoteSelect", "remoteSelects"}, field.DesignType) {
			if field.Form.RelationFields != "" && field.Form.RemoteTable != "" {
				columns, _ := getColumns(field.Form.RemoteTable)
				if err := parseJoinData(db, columns, &langEnData, &langZhData, &handlerData, &modelData, &indexVueData, field, getTableName, webTranslate); err != nil {
					return WebDir{}, "", err
				}
			}
		}

		// 模型方法
		parseModelMethods(field, &modelData)

		// 控制器/模型等文件的一些杂项属性解析
		parseSundryData(&handlerData, &indexVueData, &formVueData, field, table)

		if !slices.Contains(table.FormFields, field.Name) {
			handlerData.Attr["preExcludeFields"] = field.Name
		}
	}

	// 快速搜索提示
	langEnData["quick Search Fields"] = strings.Join(table.QuickSearchField, ",")
	langZhData["quick Search Fields"] = strings.Join(quickSearchFieldZhCnTitle, "、")
	handlerData.Attr["quickSearchField"] = strings.Join(table.QuickSearchField, ",")

	// 开启字段排序
	if _, ok := fieldsMap["weigh"]; ok {
		indexVueData.EnableDragSort = "true"
		modelData.AfterInsert = assembleStub("mixins/model/afterInsert", map[string]string{
			"field": "weight",
		}, false)
	}

	// 表格的操作列
	width := "100"
	if indexVueData.EnableDragSort == "true" {
		width = "140"
	}
	operateColumn := " label: t('Operate'), align: 'center', width: " + width + ", render: 'buttons', buttons: optButtons, operator: false"
	indexVueData.TableColumn = append(indexVueData.TableColumn, operateColumn)
	if indexVueData.EnableDragSort == "true" {
		indexVueData.OptButtons = append([]string{"weigh-sort"}, indexVueData.OptButtons...)
	}

	// 写入语言包代码
	if err := writeWebLangFile(langEnData, "en", webLangDir); err != nil {
		return WebDir{}, "", err
	}
	if err := writeWebLangFile(langZhData, "zh-cn", webLangDir); err != nil {
		return WebDir{}, "", err
	}

	// 写入index.vue代码
	indexVueData.TablePk = tablePk
	indexVueData.WebTranslate = webTranslate
	if err := writeIndexFile(indexVueData, webViewsDir, handlerFile); err != nil {
		return WebDir{}, "", err
	}

	// 写入form.vue代码
	if err := writeFormFile(formVueData, webViewsDir, fields, webTranslate); err != nil {
		return WebDir{}, "", err
	}

	// 写入模型代码
	structContent, err := writeModelFile(db, tablePk, fullTableName, tableName, modelData, modelFile)
	if err != nil {
		return WebDir{}, "", err
	}

	//写入控制器代码
	if err := writeHandlerFile(handlerData, handlerFile, structContent); err != nil {
		return WebDir{}, "", err
	}
	return webViewsDir, tableComment, err
}

// pkGoField returns the Go field name used by generated GORM structs for the
// table primary key. GORM/gen capitalizes "id" as "ID".
func pkGoField(pk string) string {
	if strings.EqualFold(pk, "id") {
		return "ID"
	}
	return utils.SnakeToCamel(pk, true)
}

func primaryKeyGoType(field model.Field) (string, error) {
	base := strings.ToLower(analyseFieldType(field))
	switch base {
	case "int", "mediumint":
		return "int32", nil
	case "bigint":
		return "int64", nil
	case "varchar", "char":
		return "string", nil
	default:
		return "", fmt.Errorf("unsupported primary key type %q for field %q; supported types are int, mediumint, bigint, varchar, and char", base, field.Name)
	}
}

func ownerGoType(field model.Field) (string, error) {
	base := strings.ToLower(analyseFieldType(field))
	switch base {
	case "tinyint", "smallint", "mediumint", "int", "integer":
		return "int32", nil
	case "bigint":
		return "int64", nil
	default:
		return "", fmt.Errorf("unsupported data-scope owner type %q for field %q", base, field.Name)
	}
}

// buildIndexProver returns a prover that checks information_schema.STATISTICS for
// an index on the owner column. It is production-only; tests can supply their
// own ProveIndex callback.
func buildIndexProver(db *gorm.DB, fullTableName string) func(string) (bool, error) {
	return func(column string) (bool, error) {
		if db == nil {
			return false, nil
		}
		var count int64
		err := db.Raw(
			"SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ? AND SEQ_IN_INDEX = 1",
			fullTableName, column,
		).Scan(&count).Error
		if err != nil {
			return false, err
		}
		return count > 0, nil
	}
}

// buildEditableColumns returns the columns that may be updated by Edit.
// It excludes the primary key, the data-scope owner, timestamp fields, and any
// fields that are not part of the form (read-only / form-excluded).
func buildEditableColumns(pk, ownerColumn string, formFields []string, fields []model.Field) []string {
	timestampFields := []string{"create_time", "createtime", "update_time", "updatetime"}
	result := make([]string, 0, len(formFields))
	for _, name := range formFields {
		if name == pk || name == ownerColumn {
			continue
		}
		if slices.Contains(timestampFields, name) {
			continue
		}
		f := searchField(fields, name)
		if f.Name != "" && (f.FormBuildExclude || f.TableBuildExclude) {
			continue
		}
		result = append(result, name)
	}
	return result
}

func joinQuotedColumns(columns []string) string {
	parts := make([]string, len(columns))
	for i, c := range columns {
		parts[i] = strconv.Quote(c)
	}
	return strings.Join(parts, ", ")
}

func buildFormFieldMarkup(formFields []string, fields []model.Field, webTranslate string, getTableName GetTableName) []string {
	result := make([]string, 0, len(formFields))
	for _, field := range fields {
		if !slices.Contains(formFields, field.Name) {
			continue
		}
		field = analyseField(field)
		result = append(result, getFormField(field, getColumnDict(field, "", webTranslate), webTranslate, getTableName))
	}
	return result
}

// 获取表主键
func getPk(fields []model.Field) string {
	pk := "id"
	for _, v := range fields {
		if v.PrimaryKey {
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
func ParseNameData(module string, tableName string, moduleType string, file string) (NameInfo, error) {
	var pathArr []string
	if file != "" {
		if err := validateRelativePathInput(file); err != nil {
			return NameInfo{}, err
		}
		file = strings.TrimSuffix(file, ".go")
		file = strings.ReplaceAll(file, ".", "/")
		file = strings.ReplaceAll(file, "\\", "/")

		redundantDir := []string{"app", module, moduleType}
		pathArr = strings.Split(file, "/")
		_, pathArr = TrimPrefix(redundantDir, pathArr)
	} else {
		if _, ok := parseNamePresets[moduleType+"/"+tableName]; ok {
			pathArr = parseNamePresets[moduleType+"/"+tableName]
		} else {
			tableName = strings.ReplaceAll(tableName, ".", "/")
			tableName = strings.ReplaceAll(tableName, "\\", "/")
			pathArr = strings.Split(tableName, "/")
		}
	}

	originalLastName := pathArr[len(pathArr)-1]
	lastName := strings.ToLower(originalLastName)
	pathArr = pathArr[:len(pathArr)-1]
	for k, v := range pathArr {
		pathArr[k] = strings.ToLower(v)
	}

	// 类名不能为内部关键字
	if slices.Contains(reservedKeywords, lastName) {
		return NameInfo{}, cErr.BadRequest("Unable to use internal variable:" + lastName)
	}

	namespace := moduleType
	if len(pathArr) > 0 {
		namespace = pathArr[len(pathArr)-1]
	}
	parseFile := filepath.Join(utils.RootPath(), "app", module, moduleType, filepath.Join(pathArr...), lastName+".go")
	if err := validateAbsolutePathUnderRoots(parseFile, filepath.Join("app", module, moduleType)); err != nil {
		return NameInfo{}, err
	}
	rootFileName := filepath.Join("app", module, moduleType, filepath.Join(pathArr...))

	info := NameInfo{
		LastName:         utils.SnakeToCamel(lastName, true),
		OriginalLastName: originalLastName,
		Path:             pathArr,
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
	var pathArr []string
	if file != "" {
		if err := validateRelativePathInput(file); err != nil {
			return WebDir{}
		}
		file = strings.TrimSuffix(file, ".go")
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

	originalLastName := pathArr[len(pathArr)-1]
	lastName := strings.ToLower(originalLastName)
	pathArr = pathArr[:len(pathArr)-1]
	for k, v := range pathArr {
		pathArr[k] = strings.ToLower(v)
	}

	webDir := WebDir{
		Path:             pathArr,
		LastName:         lastName,
		OriginalLastName: originalLastName,
	}

	if moduleType == "views" {
		webDir.Views = filepath.Join("web/src/views/backend", strings.Join(pathArr, "/"), lastName)
		if validateAbsolutePathUnderRoots(filepath.Join(utils.RootPath(), webDir.Views), "web/src/views") != nil {
			return WebDir{}
		}
	} else if moduleType == "lang" {
		webDir.Lang = append(webDir.Lang, pathArr...)
		webDir.Lang = append(webDir.Lang, lastName)
		if validateAbsolutePathUnderRoots(filepath.Join(utils.RootPath(), webDir.LangFile("en")), "web/src/lang") != nil ||
			validateAbsolutePathUnderRoots(filepath.Join(utils.RootPath(), webDir.LangFile("zh-cn")), "web/src/lang") != nil {
			return WebDir{}
		}
	}
	return webDir
}

func analyseField(field model.Field) model.Field {
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
	return field
}

// 分析字段数据类型
func analyseFieldType(field model.Field) string {
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

// 获取字段字典数据
func getDictData(dict *map[string]string, field model.Field, lang string, translationPrefix string) {
	if field.Comment == "" {
		return
	}
	comment := strings.ReplaceAll(field.Comment, "，", ",")
	comment = strings.ReplaceAll(comment, "：", ":")
	if strings.Contains(comment, ":") && strings.Contains(comment, ",") && strings.Contains(comment, "=") {
		commentArr := strings.Split(comment, ":")
		if lang == "en" {
			(*dict)[translationPrefix+field.Name] = field.Name
		} else {
			(*dict)[translationPrefix+field.Name] = commentArr[0]
		}

		items := strings.Split(commentArr[1], ",")
		for _, v := range items {
			valArr := strings.Split(v, "=")
			if len(valArr) == 2 {
				if lang == "en" {
					(*dict)[translationPrefix+field.Name+" "+valArr[0]] = field.Name + " " + valArr[0]
				} else {
					(*dict)[translationPrefix+field.Name+" "+valArr[0]] = valArr[1]
				}
			}
		}
	} else {
		if lang == "en" {
			(*dict)[translationPrefix+field.Name] = field.Name
		} else {
			(*dict)[translationPrefix+field.Name] = comment
		}
	}
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
	getDictData(&dictData, column, "zh-cn", translationPrefix)
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

func getFormField(field model.Field, columnDict map[string]string, webTranslate string, getTableName GetTableName) string {

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
		fieldHtml += " :data=\"{ content: " + getJsonFromArray(columnDict) + " }\""

	} else if field.DesignType == "textarea" {
		rows := 3
		if field.Form.Rows != 0 {
			rows = field.Form.Rows
		}

		fieldHtml += " :input-attr=\"{ rows: " + strconv.Itoa(rows) + " }\""
		fieldHtml += " @keyup.enter.stop=\"\""
		fieldHtml += " @keyup.ctrl.enter=\"baTable.onSubmit(formRef)\""

	} else if field.DesignType == "remoteSelect" || field.DesignType == "remoteSelects" {
		fName := "name"
		if field.Form.RemoteField != "" {
			fName = field.Form.RemoteField
		}
		attr := map[string]string{
			"pk":         GetRemotePk(getTableName(field.Form.RemoteTable, true), field),
			"field":      fName,
			"remote-url": GetRemoteSelectUrl(field),
		}
		fieldHtml += " :input-attr=\"" + getJsonFromArray(attr) + "\""

	} else if field.DesignType == "number" {
		step := 1
		if field.Form.Step != 0 {
			step = field.Form.Step
		}
		fieldHtml += " :input-attr=\"{ step: " + strconv.Itoa(step) + " }\""

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
		defaultValue = field.Name + ":" + buildSimpleArray(strings.Split(field.Default, ","))
	}

	if slices.Contains([]string{"weigh", "number", "float"}, field.DesignType) {
		defaultValue = field.Name + ":" + field.Default
	}
	return defaultValue
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

func getTableColumn(field model.Field, columnDict map[string]string, fieldNamePrefix string, translationPrefix string, webTranslate string) string {
	prop := ""
	if field.DesignType == "city" {
		prop = "_text"
	}

	columnStr := ""
	if field.Table.Label == "" {
		columnStr += buildTableColumnKey("label", "t("+strconv.Quote(webTranslate+translationPrefix+field.Name)+")")
	} else {
		columnStr += buildTableColumnKey("label", field.Table.Label)
	}
	columnStr += buildTableColumnKey("prop", fieldNamePrefix+field.Name+prop)
	columnStr += buildTableColumnKey("align", "center")

	// 模糊搜索增加一个placeholder
	if field.Table.Operator != "" && field.Table.Operator == "LIKE" {
		columnStr += buildTableColumnKey("operatorPlaceholder", "t('Fuzzy query')")
	}

	// 合并前端预设的字段表格属性
	if field.Table.Render != "" && field.Table.Render != "none" {
		columnStr += buildTableColumnKey("render", field.Table.Render)
	}
	if field.Table.Operator != "" {
		columnStr += buildTableColumnKey("operator", field.Table.Operator)
	}
	if field.Table.Sortable != "" {
		columnStr += buildTableColumnKey("sortable", field.Table.Sortable)
	}
	if field.Table.Width != 0 {
		columnStr += buildTableColumnKey("width", fmt.Sprintf("%v", field.Table.Width))
	}
	if field.Table.TimeFormat != "" {
		columnStr += buildTableColumnKey("timeFormat", field.Table.TimeFormat)
	}

	if field.Table.Show != "" {
		columnStr += buildTableColumnKey("show", field.Table.Show)
	}
	if field.Table.ComSearchRender != "" {
		columnStr += buildTableColumnKey("comSearchRender", field.Table.ComSearchRender)
	}
	if field.Table.Remote != "" {
		columnStr += " remote: {" + field.Table.Remote + "},"
	}

	// 需要值替换的渲染类型
	columnReplaceValue := []string{"tag", "tags", "switch"}
	if !slices.Contains([]string{"remoteSelect", "remoteSelects"}, field.DesignType) && (len(columnDict) > 0 || slices.Contains(columnReplaceValue, field.Table.Render)) {
		itemJson := ""
		for k, v := range columnDict {
			itemJson += buildTableColumnKey(k, v)
		}
		columnStr += " replaceValue: {" + strings.TrimRight(itemJson, ",") + "},"
	}
	return columnStr
}

// 关联表数据解析
func parseJoinData(db *gorm.DB, columns []model.Column, dictEn *map[string]string, dictZhCn *map[string]string, handlerData *HandlerData, modelData *ModelData, indexVueData *IndexVueData, field model.Field, getTableName GetTableName, webTranslate string) error {
	joinFields := ParseTableColumns(columns, true)
	tableName := getTableName(field.Form.RemoteTable, false)
	fullTableName := getTableName(field.Form.RemoteTable, true)
	//检查关联模型代码文件
	rootFileName, err := checkJoinMoel(db, joinFields, field, tableName, fullTableName)
	if err != nil {
		return err
	}
	if rootFileName != "" {
		field.Form.RemoteModel = rootFileName
	}

	relationFields := strings.Split(field.Form.RelationFields, ",")
	relationName := ""
	if strings.HasSuffix(field.Name, "_ids") || strings.HasSuffix(field.Name, "id") {
		relationName = strings.ReplaceAll(field.Name, "_ids", "")
		relationName = strings.ReplaceAll(relationName, "_id", "")
	} else {
		relationName = field.Name + "_table"
	}
	relationName = utils.SnakeToCamel(relationName, false)

	if field.DesignType == "remoteSelect" {
		// 关联预载入方法
		handlerData.Attr[relationName] = relationName

		// 模型方法代码
		relationPrimaryKey := "id"
		if field.Form.RemotePk != "" {
			relationPrimaryKey = field.Form.RemotePk
		}
		relationData := map[string]string{
			"relationMethod":     relationName,
			"relationMode":       "belongsTo",
			"relationPrimaryKey": relationPrimaryKey,
			"relationForeignKey": field.Name,
			"relationClassName":  "",
		}
		modelData.RelationMethodList[relationName] = assembleStub("mixins/model/belongsTo", relationData, false)

		if len(relationFields) > 0 {
			handlerData.RelationVisibleFieldList[relationData["relationMethod"]] = relationFields
		}
	} else if field.DesignType == "remoteSelects" {
		modelData.Append = append(modelData.Append, relationName)

		primaryKey := "id"
		if field.Form.RemotePk != "" {
			primaryKey = field.Form.RemotePk
		}
		labelFieldName := "name"
		if field.Form.RemoteField != "" {
			labelFieldName = field.Form.RemoteField
		}
		methodContent := assembleStub("mixins/model/getters/remoteSelectLabels", map[string]string{
			"field":          utils.SnakeToCamel(relationName, false),
			"className":      "",
			"primaryKey":     primaryKey,
			"foreignKey":     field.Name,
			"labelFieldName": labelFieldName,
		}, false)
		modelData.Methods = append(modelData.Methods, methodContent)
	}

	for _, v := range relationFields {
		joinField := searchField(joinFields, v)
		if field.Name == "" {
			continue
		}

		relationFieldPrefix := relationName + "."
		relationFieldLangPrefix := strings.ToLower(relationName) + "__"
		getDictData(dictEn, joinField, "en", relationFieldLangPrefix)
		getDictData(dictZhCn, joinField, "zh-cn", relationFieldLangPrefix)

		//不允许双击编辑的字段
		if joinField.DesignType == "switch" {
			indexVueData.DblClickNotEditColumn = append(indexVueData.DblClickNotEditColumn, field.Name)
		}

		// 列字典数据
		columnDict := getColumnDict(joinField, relationFieldLangPrefix, "")

		//表格列
		joinField.DesignType = field.DesignType
		joinField.Table.Render = "tags"
		if field.DesignType == "remoteSelects" {
			joinField.Table.Operator = "false"
			indexVueData.TableColumn = append(indexVueData.TableColumn, getTableColumn(joinField, columnDict, relationFieldPrefix, relationFieldLangPrefix, webTranslate))
			// 额外生成一个公共搜索，渲染为远程下拉的列
			joinField.Table.Label = "t('" + webTranslate + relationFieldLangPrefix + joinField.Name + "')"
			joinField.Name = field.Name
			joinField.Table.Render = ""
			joinField.Table.Show = "false"
			joinField.Table.Operator = "FIND_IN_SET"
			joinField.Table.ComSearchRender = "remoteSelect"

			primaryKey := "id"
			if field.Form.RemotePk != "" {
				primaryKey = field.Form.RemotePk
			}
			remoteTableName := getTableName(field.Form.RemoteTable, true)

			labelFieldName := "name"
			if field.Form.RemoteField != "" {
				labelFieldName = field.Form.RemoteField
			}
			itemJson := buildTableColumnKey("pk", remoteTableName+"."+primaryKey)
			itemJson += buildTableColumnKey("field", labelFieldName)
			itemJson += buildTableColumnKey("remoteUrl", GetRemoteSelectUrl(joinField))
			itemJson += buildTableColumnKey("multiple", "true")
			joinField.Table.Remote = itemJson

			indexVueData.TableColumn = append(indexVueData.TableColumn, getTableColumn(joinField, columnDict, "", relationFieldLangPrefix, webTranslate))
		} else {
			joinField.Table.Operator = "LIKE"
			indexVueData.TableColumn = append(indexVueData.TableColumn, getTableColumn(joinField, columnDict, relationFieldPrefix, relationFieldLangPrefix, webTranslate))
		}

	}
	return nil
}

// 关联表是否存在，不存在创建
func checkJoinMoel(db *gorm.DB, fields []model.Field, field model.Field, tableName, fullTableName string) (string, error) {
	rootFileName := ""

	joinModelFile, err := ParseNameData("admin", tableName, "model", field.Form.RemoteModel)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(joinModelFile.ParseFile); os.IsNotExist(err) {
		rootFileName = joinModelFile.RootFileName

		if _, err := os.Stat(joinModelFile.ParseFile); os.IsNotExist(err) {
			formFields := make([]string, 0, len(fields))
			columnFields := make([]string, 0, len(fields))
			for _, joinField := range fields {
				columnFields = append(columnFields, joinField.Name)
				if !joinField.PrimaryKey {
					formFields = append(formFields, joinField.Name)
				}
			}
			joinTable := model.Table{
				Name: tableName, ModelFile: field.Form.RemoteModel,
				FormFields: formFields, ColumnFields: columnFields,
			}
			joinGetTableName := func(name string, full bool) string {
				if full {
					return fullTableName
				}
				return name
			}
			joinModelData, _, _, _, _, _, _, _, joinTablePk, _, _, err := prepareGenerationData(joinTable, fields, nil, joinGetTableName, buildIndexProver(db, fullTableName))
			if err != nil {
				return "", err
			}
			for _, v := range fields {
				parseModelMethods(v, &joinModelData)
			}
			if _, err := writeModelFile(db, joinTablePk, fullTableName, tableName, joinModelData, joinModelFile); err != nil {
				return "", err
			}
		}
	}
	return rootFileName, nil
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
func parseSundryData(handlerData *HandlerData, indexVueData *IndexVueData, formVueData *FormVueData, field model.Field, table model.Table) {
	if field.DesignType == "editor" {
		formVueData.BigDialog = "true"
		handlerData.FilterRule = append(handlerData.FilterRule, "clean_xss")
	}

	//默认排序字段
	if table.DefaultSortField != "" && table.DefaultSortType != "" {
		defaultSortField := table.DefaultSortField + "," + table.DefaultSortType
		if defaultSortField == "id,desc" {
			handlerData.Attr["defaultSortField"] = ""
		} else {
			handlerData.Attr["defaultSortField"] = defaultSortField
			indexVueData.DefaultOrder = buildDefaultOrder(table.DefaultSortField, table.DefaultSortType)
		}
	}
}

func buildDefaultOrder(field string, sortType string) string {
	if field != "" && sortType != "" {
		defaultOrderStub := map[string]string{
			"prop":  field,
			"order": sortType,
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
	//获取转义编码后的值
	return value
}

func Tab(num int) string {
	return strings.Repeat(" ", 4*num)
}

func buildTableColumnKey(key string, val string) string {
	itemJson := ""
	key = formatObjectKey(key)
	if val == "false" || val == "true" {
		itemJson = " " + key + ": " + val + ","
	} else if key == "width" || key == "buttons" || translationCallRE.MatchString(val) {
		itemJson = " " + key + ": " + val + ","
	} else {
		itemJson = " " + key + ": " + strconv.Quote(val) + ","
	}
	return itemJson
}

var translationCallRE = regexp.MustCompile(`^t\(("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')\)$`)

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
			jsonStr += keyStr + quote + v + quote + ","
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
