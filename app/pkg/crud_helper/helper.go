package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	crudmodel "go-build-admin/app/admin/model/crud"
	"go-build-admin/app/pkg/data_scope"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/utils"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// 生成表
func GenerateFile(table crudmodel.Table, fields []crudmodel.Field, getTableName GetTableName, getColumns GetColumns, db *gorm.DB) (WebDir, string, error) {
	if err := ValidateGenerationInput(table, fields); err != nil {
		return WebDir{}, "", err
	}
	return GenerateFileWithRouteRegistrar(table, fields, table.DataScope, getTableName, getColumns, db, nil)
}

// prepareGenerationData resolves data-scope policy and initializes the model/handler
// data structures used by both production generation and compile-only tests.
func prepareGenerationData(table crudmodel.Table, fields []crudmodel.Field, dsConfig *data_scope.Config, getTableName GetTableName, proveIndex func(string) (bool, error)) (ModelData, HandlerData, NameInfo, NameInfo, WebDir, WebDir, string, string, string, string, string, error) {
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
	modelData.ModelFieldType = buildModelFieldTypeOverrides(fields)
	modelData.BeforeInsertMixins = map[string]string{}
	modelData.CityTextFields = collectCityTextFields(fields)

	// 控制器数据
	handlerData := HandlerData{}
	handlerData.Namespace = handlerFile.Namespace
	handlerData.ModelNamespace = modelData.Namespace
	modelImportSegments := append([]string{"go-build-admin", "app", module, "model"}, modelFile.Path...)
	handlerData.ModelImportPath = strings.Join(modelImportSegments, "/")
	handlerData.ClassName = handlerFile.LastName
	handlerData.ModelName = modelData.ClassName
	handlerData.ModelVar = strings.ToLower(string(modelFile.LastName[0])) + modelFile.LastName[1:]
	handlerData.RouteName = routeNameFromRelativePath(table.GenerateRelativePath, handlerData.ClassName)
	handlerData.PkGoType = modelData.PkGoType
	handlerData.PkJSONName = tablePk
	handlerData.TableComment = tableComment

	handlerData.Import = []string{}
	handlerData.Attr = map[string]string{}
	handlerData.Methods = []string{}
	handlerData.ParamTypeOverrides = map[string]string{}
	handlerData.PartialEditFields = buildPartialEditFields(fields)

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
	for _, name := range []string{"create_time", "createtime", "update_time", "updatetime"} {
		if searchField(fields, name).Name == "" {
			continue
		}
		if !slices.Contains(handlerData.ExcludeParamFields, name) {
			handlerData.ExcludeParamFields = append(handlerData.ExcludeParamFields, name)
		}
	}

	return modelData, handlerData, modelFile, handlerFile, webViewsDir, webLangDir, webTranslate, tableComment, tablePk, tableName, fullTableName, nil
}

// GenerateFileWithDataScope generates CRUD files using the persisted data-scope
// configuration. A nil dsConfig preserves legacy auto-detection behavior.
func GenerateFileWithDataScope(table crudmodel.Table, fields []crudmodel.Field, dsConfig *data_scope.Config, getTableName GetTableName, getColumns GetColumns, db *gorm.DB) (WebDir, string, error) {
	return GenerateFileWithRouteRegistrar(table, fields, dsConfig, getTableName, getColumns, db, nil)
}

func GenerateFileWithRouteRegistrar(table crudmodel.Table, fields []crudmodel.Field, dsConfig *data_scope.Config, getTableName GetTableName, getColumns GetColumns, db *gorm.DB, registrar func(method, path string)) (WebDir, string, error) {
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
	indexVueData.RouteName = handlerData.RouteName
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
		for name, typeName := range buildHandlerParamTypeOverrides([]crudmodel.Field{field}) {
			handlerData.ParamTypeOverrides[name] = typeName
		}

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

		// 表格列。For multi remoteSelects with relation fields, retain the raw FK
		// search column but hide it; relation display columns are added below.
		field = prepareGeneratedColumnField(field)
		if slices.Contains(table.ColumnFields, field.Name) {
			if field.Table.ComSearchRender == "remoteSelect" && field.Form.RemoteTable != "" {
				field.Table.Remote = buildRemoteSearchMetadata(field, getTableName)
			}
			indexVueData.TableColumn = append(indexVueData.TableColumn, getTableColumn(field, columnDict, "", "", webTranslate))
		}

		// 关联表数据解析
		if slices.Contains([]string{"remoteSelect", "remoteSelects"}, field.DesignType) {
			if field.Form.RelationFields != "" && field.Form.RemoteTable != "" {
				columns, err := getColumns(field.Form.RemoteTable)
				if err != nil {
					return WebDir{}, "", fmt.Errorf("remote relation %q for field %q: %w", field.Form.RemoteTable, field.Name, err)
				}
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
	finalizeRelationMetadata(&modelData)

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
	indexVueData.TableColumn = append(indexVueData.TableColumn, buildOperateColumn(indexVueData.EnableDragSort == "true"))
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

func routeNameFromRelativePath(relativePath, fallback string) string {
	parts := strings.Split(strings.Trim(relativePath, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[len(parts)-1] == "" {
		return lowerFirst(fallback)
	}
	return strings.Join(parts[:len(parts)-1], ".") + "." + utils.SnakeToCamel(parts[len(parts)-1], true)
}

func primaryKeyGoType(field crudmodel.Field) (string, error) {
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

func ownerGoType(field crudmodel.Field) (string, error) {
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
func buildEditableColumns(pk, ownerColumn string, formFields []string, fields []crudmodel.Field) []string {
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
		if f.Name != "" && f.FormBuildExclude {
			continue
		}
		result = append(result, name)
	}
	return result
}

func buildRemoteSearchMetadata(field crudmodel.Field, getTableName GetTableName) string {
	remoteField := field.Form.RemoteField
	if remoteField == "" {
		remoteField = "name"
	}
	remote := buildTableColumnKey("pk", GetRemotePk(getTableName(field.Form.RemoteTable, true), field))
	remote += buildTableColumnKey("field", remoteField)
	remote += buildTableColumnKey("remoteUrl", GetRemoteSelectUrl(field))
	if field.DesignType == "remoteSelects" {
		remote += buildTableColumnKey("multiple", "true")
	}
	return remote
}

func buildOperateColumn(enableDragSort bool) string {
	width := "100"
	if enableDragSort {
		width = "140"
	}
	return " label: t('Operate'), align: 'center', width: " + width + ", fixed: 'right', render: 'buttons', buttons: optButtons, operator: false"
}

func prepareGeneratedColumnField(field crudmodel.Field) crudmodel.Field {
	if slices.Contains([]string{"remoteSelect", "remoteSelects"}, field.DesignType) && field.Form.RemoteTable != "" && strings.TrimSpace(field.Form.RelationFields) != "" && field.Table.Show == "" {
		field.Table.Show = "false"
	}
	return field
}

func buildPartialEditFields(fields []crudmodel.Field) string {
	partialEditFields := make([]string, 0)
	for _, field := range fields {
		if analyseField(field).DesignType == "switch" {
			partialEditFields = append(partialEditFields, strconv.Quote(field.Name)+": true")
		}
	}
	return strings.Join(partialEditFields, ", ")
}

func joinQuotedColumns(columns []string) string {
	parts := make([]string, len(columns))
	for i, c := range columns {
		parts[i] = strconv.Quote(c)
	}
	return strings.Join(parts, ", ")
}

func buildFormFieldMarkup(formFields []string, fields []crudmodel.Field, webTranslate string, getTableName GetTableName) []string {
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
func getPk(fields []crudmodel.Field) string {
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
	explicitPath := file != ""
	if file != "" {
		if err := validateRelativePathInput(file); err != nil {
			return NameInfo{}, err
		}
		var normalizeErr error
		file, normalizeErr = normalizeLogicalPath(file)
		if normalizeErr != nil {
			return NameInfo{}, normalizeErr
		}

		redundantDir := []string{"app", module, moduleType}
		pathArr = strings.Split(file, "/")
		_, pathArr = TrimPrefix(redundantDir, pathArr)
	} else {
		if _, ok := parseNamePresets[moduleType+"/"+tableName]; ok {
			pathArr = parseNamePresets[moduleType+"/"+tableName]
		} else {
			normalized, normalizeErr := normalizeLogicalPath(tableName)
			if normalizeErr != nil {
				return NameInfo{}, normalizeErr
			}
			pathArr = strings.Split(normalized, "/")
		}
	}

	originalLastName := pathArr[len(pathArr)-1]
	lastName := strings.ToLower(originalLastName)
	if explicitPath {
		lastName = originalLastName
	}
	pathArr = pathArr[:len(pathArr)-1]
	for k, v := range pathArr {
		if !explicitPath {
			pathArr[k] = strings.ToLower(v)
		}
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
		var normalizeErr error
		file, normalizeErr = normalizeLogicalPath(file)
		if normalizeErr != nil {
			return WebDir{}
		}

		redundantDir := []string{"web", "src", "views", "backend"}
		pathArr = strings.Split(file, "/")
		_, pathArr = TrimPrefix(redundantDir, pathArr)

	} else {
		if _, ok := parseWebDirPresets[moduleType+"/"+tableName]; ok {
			pathArr = parseWebDirPresets[moduleType+"/"+tableName]
		} else {
			normalized, normalizeErr := normalizeLogicalPath(tableName)
			if normalizeErr != nil {
				return WebDir{}
			}
			parts := strings.Split(normalized, "/")
			if len(parts) == 1 {
				parts = strings.Split(parts[0], "_")
			}
			if len(parts) >= 3 {
				parts = append(parts[:len(parts)-2], utils.SnakeToCamel(strings.Join(parts[len(parts)-2:], "_"), false))
			}
			pathArr = parts
		}
	}

	originalLastName := pathArr[len(pathArr)-1]
	// 对齐上游:lastName 仅首字母小写(lcfirst),保留驼峰;
	// 否则显式路径 country/languageContent 的 views 目录会被压成 languagecontent,
	// 与菜单名(取 OriginalLastName)不一致
	lastName := originalLastName
	if lastName != "" {
		lastName = strings.ToLower(lastName[:1]) + lastName[1:]
	}
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

func analyseField(field crudmodel.Field) crudmodel.Field {
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

func buildHandlerParamTypeOverrides(fields []crudmodel.Field) map[string]string {
	overrides := make(map[string]string)
	for _, field := range fields {
		designType := field.DesignType
		dbType := strings.ToLower(analyseFieldType(field))
		switch {
		case isCanonicalTimeField(field.Name):
			// Auto-maintained integer timestamps stay plain int64 in model/handler DTOs.
		case isBooleanStorageField(field):
			overrides[field.Name] = "validate.FlexBool"
		case designType == "year" || dbType == "year":
			overrides[field.Name] = "validate.FlexYear"
		case slices.Contains(dtStringToArray, designType):
			overrides[field.Name] = "validate.CommaJoined"
		case designType == "array":
			overrides[field.Name] = "validate.KeyValueArray"
		case designType == "datetime" && slices.Contains([]string{"datetime", "timestamp"}, dbType):
			overrides[field.Name] = "validate.FlexDateTime"
		case designType == "date":
			overrides[field.Name] = "validate.FlexDate"
		case designType == "time":
			overrides[field.Name] = "validate.FlexClock"
		case field.OriginalDesignType == "timestamp" && slices.Contains([]string{"bigint", "int", "mediumint", "smallint", "tinyint"}, dbType):
			overrides[field.Name] = timestampAdapterType(field.Name)
		}
	}
	return overrides
}

func buildModelFieldTypeOverrides(fields []crudmodel.Field) map[string]string {
	overrides := make(map[string]string)
	for _, rawField := range fields {
		field := analyseField(rawField)
		dbType := strings.ToLower(analyseFieldType(field))
		switch {
		case isCanonicalTimeField(field.Name):
			// Auto-maintained integer timestamps stay plain int64 (not FlexUnixTime).
		case isBooleanStorageField(field):
			overrides[field.Name] = "validate.FlexBool"
		case field.DesignType == "year" || dbType == "year":
			overrides[field.Name] = "validate.FlexYear"
		case dbType == "time":
			if field.DesignType == "time" {
				overrides[field.Name] = "validate.FlexClock"
			} else {
				overrides[field.Name] = "string"
			}
		case slices.Contains(dtStringToArray, field.DesignType):
			overrides[field.Name] = "validate.CommaJoined"
		case field.DesignType == "array":
			overrides[field.Name] = "validate.KeyValueArray"
		case field.DesignType == "datetime" && slices.Contains([]string{"datetime", "timestamp"}, dbType):
			overrides[field.Name] = "validate.FlexDateTime"
		case field.DesignType == "date" && dbType == "date":
			overrides[field.Name] = "validate.FlexDate"
		case field.DesignType == "time" && dbType == "time":
			overrides[field.Name] = "validate.FlexClock"
		case field.OriginalDesignType == "timestamp" && slices.Contains([]string{"bigint", "int", "mediumint", "smallint", "tinyint"}, dbType):
			overrides[field.Name] = timestampAdapterType(field.Name)
		}
	}
	return overrides
}

func timestampAdapterType(fieldName string) string {
	// Canonical names are skipped before this helper is called.
	// Non-canonical integer timestamp design fields keep formatted JSON output.
	return "validate.FlexFormattedUnixTime"
}

func isBooleanStorageField(field crudmodel.Field) bool {
	if !strings.EqualFold(strings.TrimSpace(field.Type), "tinyint") {
		return false
	}
	dataType := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(field.DataType), " ", ""))
	if strings.HasPrefix(dataType, "tinyint(1") {
		return true
	}
	if field.Length == 1 {
		return true
	}
	return false
}

// 分析字段数据类型
func analyseFieldType(field crudmodel.Field) string {
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
func getDictData(dict *map[string]string, field crudmodel.Field, lang string, translationPrefix string) {
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

func getColumnDict(column crudmodel.Field, translationPrefix string, webTranslate string) map[string]string {
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

func getFormField(field crudmodel.Field, columnDict map[string]string, webTranslate string, getTableName GetTableName) string {

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
		step := float64(1)
		if field.Form.Step != 0 {
			step = field.Form.Step
		}
		fieldHtml += " :input-attr=\"{ step: " + formatJSNumber(step) + " }\""

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

func formatJSNumber(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// getFieldDefault 生成 index.vue defaultItems 的单项,语义对齐上游
// BuildAdmin v2 Crud::getFormField 的默认值处理。返回空字符串表示不生成该项。
func getFieldDefault(field crudmodel.Field) string {
	// array 类型固定为空数组
	if field.DesignType == "array" {
		return field.Name + ": []"
	}

	// editor 始终生成默认值项
	if field.DesignType == "editor" {
		def := ""
		if field.DefaultType == "INPUT" {
			def = field.Default
		}
		return field.Name + ":" + getQuote(def) + def + getQuote(def)
	}

	// 仅 INPUT 类型的默认值进入 defaultItems
	if field.DefaultType != "INPUT" {
		return ""
	}

	// 多值字符串拆分为数组
	if slices.Contains(dtStringToArray, field.DesignType) && strings.Contains(field.Default, ",") {
		return field.Name + ":" + buildSimpleArray(strings.Split(field.Default, ","))
	}

	// 数字类型输出原始数值;0 为无意义默认值
	if slices.Contains([]string{"number", "float"}, field.DesignType) {
		num, err := strconv.ParseFloat(field.Default, 64)
		if err != nil || num == 0 {
			return ""
		}
		return field.Name + ":" + strconv.FormatFloat(num, 'f', -1, 64)
	}

	// switch/remoteSelect 的 0 为无意义默认值
	if slices.Contains([]string{"switch", "remoteSelect"}, field.DesignType) && field.Default == "0" {
		return ""
	}

	return field.Name + ":" + getQuote(field.Default) + field.Default + getQuote(field.Default)
}

func GetRemotePk(fullTableName string, field crudmodel.Field) string {
	if strings.Contains(field.Form.RemotePk, ".") {
		return field.Form.RemotePk
	}
	name := fullTableName
	if field.Form.RemotePrimaryTableAlias != "" {
		name = field.Form.RemotePrimaryTableAlias
	}
	if field.Form.RemotePk == "" {
		return name + ".id"
	}
	return name + "." + field.Form.RemotePk
}

// GetRemoteSelectUrl 对齐上游:crud 来源且指定了控制器时由控制器推导 URL,
// 否则使用手动填写的 remote-url。优先从同目录 RouteRegistrar 反查，
// 兼容尚未迁移的 handler 时再从旧 router.go 注册信息反查，最后按路径回退。
func GetRemoteSelectUrl(field crudmodel.Field) string {
	if field.Form.RemoteSourceConfigType != "custom" && field.Form.RemoteController != "" {
		if url := routeIndexURLForController(field.Form.RemoteController); url != "" {
			return url
		}
		controller, err := normalizeLogicalPath(field.Form.RemoteController)
		if err != nil {
			return field.Form.RemoteUrl
		}
		redundantDir := []string{"app", "admin", "handler"}
		pathArr := strings.Split(controller, "/")
		_, pathArr = TrimPrefix(redundantDir, pathArr)
		if url := strings.Join(pathArr, "."); url != "" {
			return "/admin/" + url + "/index"
		}
	}
	return field.Form.RemoteUrl
}

// routeIndexURLForController 由控制器文件推导同目录 registrar 文件并读取
// route 常量；尚未迁移的 handler 则回退到 router.go 中的旧注册。
func routeIndexURLForController(controller string) string {
	normalized, err := normalizeLogicalPath(controller)
	if err != nil {
		return ""
	}
	stem := filepath.Base(normalized)
	if stem == "" {
		return ""
	}
	registrarPath := filepath.Join(utils.RootPath(), strings.TrimSuffix(normalized, filepath.Ext(normalized))+"_route.go")
	if data, err := os.ReadFile(registrarPath); err == nil {
		re := regexp.MustCompile(`const\s+\w+Route\s*=\s*"([^"]+)"`)
		if m := re.FindSubmatch(data); m != nil {
			return "/admin/" + string(m[1]) + "/index"
		}
	}
	if registrarPath = findRouteRegistrarPath(filepath.Join(utils.RootPath(), "app/admin/handler"), stem); registrarPath != "" {
		if data, err := os.ReadFile(registrarPath); err == nil {
			re := regexp.MustCompile(`const\s+\w+Route\s*=\s*"([^"]+)"`)
			if m := re.FindSubmatch(data); m != nil {
				return "/admin/" + string(m[1]) + "/index"
			}
		}
	}
	handlerVar := utils.SnakeToCamel(stem, false) + "Handler"
	data, err := os.ReadFile(filepath.Join(utils.RootPath(), "router", "router.go"))
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`adminRouter\.GET\("([^"]+)/index",\s*` + regexp.QuoteMeta(handlerVar) + `\.Index\)`)
	if m := re.FindSubmatch(data); m != nil {
		return "/admin/" + string(m[1]) + "/index"
	}
	return ""
}

func findRouteRegistrarPath(root, stem string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if entry.IsDir() {
			if registrar := findRouteRegistrarPath(path, stem); registrar != "" {
				return registrar
			}
			continue
		}
		if strings.HasSuffix(entry.Name(), "_route.go") && strings.TrimSuffix(entry.Name(), "_route.go") == stem {
			return path
		}
	}
	return ""
}

func getTableColumn(field crudmodel.Field, columnDict map[string]string, fieldNamePrefix string, translationPrefix string, webTranslate string) string {
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
	if len(field.Table.ComSearchInputAttr) > 0 {
		columnStr += " comSearchInputAttr: " + getJsonFromAny(field.Table.ComSearchInputAttr) + ","
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

// parseJoinData validates and records one slim relation, then adds its nested
// display columns. Relation labels never use a JOIN or relation-side scope.
func parseJoinData(_ *gorm.DB, columns []model.Column, dictEn *map[string]string, dictZhCn *map[string]string, _ *HandlerData, modelData *ModelData, indexVueData *IndexVueData, field crudmodel.Field, _ GetTableName, webTranslate string) error {
	if !slices.Contains([]string{"remoteSelect", "remoteSelects"}, field.DesignType) {
		return nil
	}
	joinFields := ParseTableColumns(columns, true)
	relationFields := strings.Split(field.Form.RelationFields, ",")
	relationName := relationNameForField(field.Name)
	relation, err := buildRelationMetadata(joinFields, field, modelData.ClassName)
	if err != nil {
		return err
	}
	if err := appendRelationMetadata(modelData, relation); err != nil {
		return err
	}

	for _, v := range relationFields {
		v = strings.TrimSpace(v)
		joinField := searchField(joinFields, v)
		if joinField.Name == "" {
			return fmt.Errorf("unknown relation field %q on remote table %q for field %q", v, field.Form.RemoteTable, field.Name)
		}
		if len(relationFields) == 1 && field.Table.Label != "" {
			joinField.Table.Label = field.Table.Label
		}

		relationFieldPrefix := relationName + "."
		relationFieldLangPrefix := strings.ToLower(relationName) + "__"
		getDictData(dictEn, joinField, "en", relationFieldLangPrefix)
		getDictData(dictZhCn, joinField, "zh-cn", relationFieldLangPrefix)

		if joinField.DesignType == "switch" {
			indexVueData.DblClickNotEditColumn = append(indexVueData.DblClickNotEditColumn, field.Name)
		}

		columnDict := getColumnDict(joinField, relationFieldLangPrefix, "")
		joinField.DesignType = field.DesignType
		joinField.Table.Render = "tags"
		joinField.Table.Operator = "false"
		indexVueData.TableColumn = append(indexVueData.TableColumn, getTableColumn(joinField, columnDict, relationFieldPrefix, relationFieldLangPrefix, webTranslate))
	}
	return nil
}

func buildRelationMetadata(joinFields []crudmodel.Field, field crudmodel.Field, className string) (RelationMetadata, error) {
	remoteTable := field.Form.RemoteTable
	if err := data_scope.ValidateIdentifier(remoteTable); err != nil {
		return RelationMetadata{}, fmt.Errorf("invalid remote table %q for field %q: %w", remoteTable, field.Name, err)
	}
	remotePK := field.Form.RemotePk
	if remotePK == "" {
		remotePK = "id"
	}
	if err := data_scope.ValidateIdentifier(remotePK); err != nil {
		return RelationMetadata{}, fmt.Errorf("%s relation %q has unsupported remote primary key %q: %w", field.DesignType, field.Name, remotePK, err)
	}
	multi := field.DesignType == "remoteSelects"
	if multi && !isMultiRelationStorage(field) {
		return RelationMetadata{}, fmt.Errorf("remoteSelects relation %q requires string-family CSV storage, got %q", field.Name, analyseFieldType(field))
	}
	if multi && strings.TrimSpace(field.Form.RelationFields) == "" {
		return RelationMetadata{}, fmt.Errorf("remoteSelects relation %q requires relationFields for deferred label enrichment", field.Name)
	}
	pkColumn := searchField(joinFields, remotePK)
	if pkColumn.Name == "" {
		return RelationMetadata{}, fmt.Errorf("remote primary key %q not found on remote table %q for field %q", remotePK, remoteTable, field.Name)
	}

	relationName := relationNameForField(field.Name)
	metadata := RelationMetadata{
		FieldName:       field.Name,
		RelationName:    relationName,
		RelationGoField: utils.SnakeToCamel(relationName, true),
		DTOName:         className + utils.SnakeToCamel(relationName, true) + "Relation",
		RemoteTable:     remoteTable,
		RemotePK:        remotePK,
		RemotePKGoField: generatedGoFieldName(remotePK),
		RemotePKType:    relationColumnGoTypeFromField(pkColumn),
		RowDTOName:      className + utils.SnakeToCamel(relationName, true) + "RelationRow",
		Multi:           multi,
	}
	if !slices.Contains([]string{"int32", "int64", "string"}, metadata.RemotePKType) {
		return RelationMetadata{}, fmt.Errorf("%s relation %q has unsupported remote primary key type %q; supported types are int32, int64, and string", field.DesignType, field.Name, metadata.RemotePKType)
	}
	metadata.Fields = append(metadata.Fields, relationDTOField(pkColumn))

	seen := map[string]bool{remotePK: true}
	for _, rawName := range strings.Split(field.Form.RelationFields, ",") {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return RelationMetadata{}, fmt.Errorf("empty relation field for field %q", field.Name)
		}
		if err := data_scope.ValidateIdentifier(name); err != nil {
			return RelationMetadata{}, fmt.Errorf("invalid relation field %q for field %q: %w", name, field.Name, err)
		}
		joinField := searchField(joinFields, name)
		if joinField.Name == "" {
			return RelationMetadata{}, fmt.Errorf("unknown relation field %q on remote table %q for field %q", name, remoteTable, field.Name)
		}
		if multi {
			alreadyPayload := false
			for _, payloadField := range metadata.PayloadFields {
				if payloadField.ColumnName == name {
					alreadyPayload = true
					break
				}
			}
			if !alreadyPayload {
				metadata.PayloadFields = append(metadata.PayloadFields, relationDTOField(joinField))
			}
		}
		if !seen[name] {
			metadata.Fields = append(metadata.Fields, relationDTOField(joinField))
			seen[name] = true
		}
	}
	if !multi {
		metadata.PayloadFields = metadata.Fields
	}
	return metadata, nil
}

func isMultiRelationStorage(field crudmodel.Field) bool {
	base := strings.ToLower(analyseFieldType(field))
	return slices.Contains([]string{"char", "varchar", "text", "tinytext", "mediumtext", "longtext", "set"}, base)
}

func appendRelationMetadata(modelData *ModelData, relation RelationMetadata) error {
	for _, existing := range modelData.Relations {
		if existing.RelationName == relation.RelationName || existing.DTOName == relation.DTOName || (relation.Multi && existing.RowDTOName == relation.RowDTOName) {
			return fmt.Errorf("relation name/DTO collision for %q on field %q", relation.RelationName, relation.FieldName)
		}
	}
	modelData.Relations = append(modelData.Relations, relation)
	return nil
}

func relationDTOField(field crudmodel.Field) RelationDTOField {
	return RelationDTOField{
		ColumnName: field.Name,
		GoName:     generatedGoFieldName(field.Name),
		GoType:     relationColumnGoTypeFromField(field),
		JSONName:   field.Name,
		Nullable:   field.Null,
	}
}

func relationColumnGoType(field model.Column) string {
	base := strings.ToLower(field.DATA_TYPE)
	if base == "" {
		base = strings.ToLower(field.COLUMN_TYPE)
		if index := strings.IndexByte(base, '('); index >= 0 {
			base = base[:index]
		}
		base = strings.TrimSpace(strings.TrimSuffix(base, " unsigned"))
	}
	switch base {
	case "tinyint", "smallint", "mediumint", "int", "integer":
		return "int32"
	case "bigint":
		return "int64"
	case "float", "double", "decimal", "numeric":
		return "float64"
	case "bool", "boolean":
		return "bool"
	case "date", "datetime", "timestamp", "time":
		return "time.Time"
	default:
		return "string"
	}
}

func relationColumnGoTypeFromField(field crudmodel.Field) string {
	column := model.Column{DATA_TYPE: field.Type, COLUMN_TYPE: field.DataType}
	return relationColumnGoType(column)
}

func generatedGoFieldName(name string) string {
	goName := utils.SnakeToCamel(name, true)
	if strings.HasSuffix(goName, "Ids") {
		return strings.TrimSuffix(goName, "Ids") + "IDs"
	}
	if strings.HasSuffix(goName, "Id") {
		return strings.TrimSuffix(goName, "Id") + "ID"
	}
	return goName
}

func finalizeRelationMetadata(modelData *ModelData) {
	modelData.RelationStructs = ""
	modelData.RelationFields = ""
	modelData.RelationLoaders = ""
	modelData.RelationNeedsTime = false
	modelData.RelationNeedsMulti = false
	modelData.RelationNeedsMultiNumeric = false
	if len(modelData.Relations) == 0 {
		return
	}

	var structs strings.Builder
	var fields strings.Builder
	var loaders strings.Builder
	for _, relation := range modelData.Relations {
		fields.WriteString("\t")
		fields.WriteString(relation.RelationGoField)
		fields.WriteString(" *")
		fields.WriteString(relation.DTOName)
		fields.WriteString(" `gorm:\"-\" json:\"")
		fields.WriteString(relation.RelationName)
		fields.WriteString("\"`\n")

		structs.WriteString("type ")
		if relation.Multi {
			modelData.RelationNeedsMulti = true
			if relation.RemotePKType != "string" {
				modelData.RelationNeedsMultiNumeric = true
			}
			structs.WriteString(relation.RowDTOName)
		} else {
			structs.WriteString(relation.DTOName)
		}
		structs.WriteString(" struct {\n")
		for _, field := range relation.Fields {
			structs.WriteString("\t")
			structs.WriteString(field.GoName)
			structs.WriteString(" ")
			if field.Nullable {
				structs.WriteString("*")
			}
			structs.WriteString(field.GoType)
			structs.WriteString(" `gorm:\"column:")
			structs.WriteString(field.ColumnName)
			structs.WriteString("\" json:\"")
			structs.WriteString(field.JSONName)
			structs.WriteString("\"`\n")
			if field.GoType == "time.Time" {
				modelData.RelationNeedsTime = true
			}
		}
		structs.WriteString("}\n\n")
		if relation.Multi {
			structs.WriteString("type ")
			structs.WriteString(relation.DTOName)
			structs.WriteString(" struct {\n")
			for _, field := range relation.PayloadFields {
				structs.WriteString("\t")
				structs.WriteString(field.GoName)
				structs.WriteString(" []*")
				structs.WriteString(field.GoType)
				structs.WriteString(" `json:\"")
				structs.WriteString(field.JSONName)
				structs.WriteString("\"`\n")
			}
			structs.WriteString("}\n\n")
		}

		loaders.WriteString(renderRelationLoader(*modelData, relation))
	}
	loaders.WriteString(renderRelationLoaderAggregator(*modelData))
	modelData.RelationStructs = structs.String()
	modelData.RelationFields = fields.String()
	modelData.RelationLoaders = loaders.String()
}

func renderRelationLoader(modelData ModelData, relation RelationMetadata) string {
	if relation.Multi {
		return renderMultiRelationLoader(modelData, relation)
	}
	var b strings.Builder
	methodName := "load" + relation.RelationGoField + "Relations"
	rowField := generatedGoFieldName(relation.FieldName)
	b.WriteString("func (s *")
	b.WriteString(modelData.ClassName)
	b.WriteString("Model) ")
	b.WriteString(methodName)
	b.WriteString("(ctx *gin.Context, rows *[]")
	b.WriteString(modelData.ClassName)
	b.WriteString(") error {\n")
	b.WriteString("\tif len(*rows) == 0 { return nil }\n")
	b.WriteString("\tkeys := make([]")
	b.WriteString(relation.RemotePKType)
	b.WriteString(", 0, len(*rows))\n")
	b.WriteString("\tseen := make(map[")
	b.WriteString(relation.RemotePKType)
	b.WriteString("]struct{}, len(*rows))\n")
	b.WriteString("\tfor i := range *rows { key := ")
	b.WriteString(relation.RemotePKType)
	b.WriteString("((*rows)[i].")
	b.WriteString(rowField)
	b.WriteString("); if _, ok := seen[key]; ok { continue }; seen[key] = struct{}{}; keys = append(keys, key) }\n")
	b.WriteString("\tif len(keys) == 0 { return nil }\n")
	b.WriteString("\trelated := make([]")
	b.WriteString(relation.DTOName)
	b.WriteString(", 0)\n")
	b.WriteString("\tif err := s.DBFor(ctx).Table(s.config.Database.Prefix + ")
	b.WriteString(strconv.Quote(relation.RemoteTable))
	b.WriteString(").Select(")
	for i, field := range relation.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(field.ColumnName))
	}
	b.WriteString(").Where(")
	b.WriteString(strconv.Quote(relation.RemotePK + " IN ?"))
	b.WriteString(", keys).Find(&related).Error; err != nil { return err }\n")
	b.WriteString("\tbyKey := make(map[")
	b.WriteString(relation.RemotePKType)
	b.WriteString("]*")
	b.WriteString(relation.DTOName)
	b.WriteString(", len(related))\n")
	b.WriteString("\tfor i := range related { byKey[related[i].")
	b.WriteString(relation.RemotePKGoField)
	b.WriteString("] = &related[i] }\n")
	b.WriteString("\tfor i := range *rows { if relation, ok := byKey[")
	b.WriteString(relation.RemotePKType)
	b.WriteString("((*rows)[i].")
	b.WriteString(rowField)
	b.WriteString(")]; ok { (*rows)[i].")
	b.WriteString(relation.RelationGoField)
	b.WriteString(" = relation } }\n")
	b.WriteString("\treturn nil\n}\n\n")
	return b.String()
}

func renderMultiRelationLoader(modelData ModelData, relation RelationMetadata) string {
	var b strings.Builder
	methodName := "load" + relation.RelationGoField + "Relations"
	rowField := generatedGoFieldName(relation.FieldName)
	b.WriteString("func (s *")
	b.WriteString(modelData.ClassName)
	b.WriteString("Model) ")
	b.WriteString(methodName)
	b.WriteString("(ctx *gin.Context, rows *[]")
	b.WriteString(modelData.ClassName)
	b.WriteString(") error {\n")
	b.WriteString("\ttype relationRef struct { rowIndex int; valueIndex int; key ")
	b.WriteString(relation.RemotePKType)
	b.WriteString(" }\n")
	b.WriteString("\tparseKey := func(token string) (")
	b.WriteString(relation.RemotePKType)
	b.WriteString(", bool) {\n")
	b.WriteString("\t\ttoken = strings.TrimSpace(token)\n")
	zero := "0"
	if relation.RemotePKType == "string" {
		zero = `""`
	}
	b.WriteString("\t\tif token == \"\" { return ")
	b.WriteString(zero)
	b.WriteString(", false }\n")
	if relation.RemotePKType == "string" {
		b.WriteString("\t\treturn token, true\n")
	} else {
		bits := "64"
		if relation.RemotePKType == "int32" {
			bits = "32"
		}
		b.WriteString("\t\tvalue, err := strconv.ParseInt(token, 10, ")
		b.WriteString(bits)
		b.WriteString(")\n")
		b.WriteString("\t\tif err != nil { return ")
		b.WriteString(zero)
		b.WriteString(", false }\n")
		b.WriteString("\t\treturn ")
		b.WriteString(relation.RemotePKType)
		b.WriteString("(value), true\n")
	}
	b.WriteString("\t}\n")
	b.WriteString("\tpayloads := make([]*")
	b.WriteString(relation.DTOName)
	b.WriteString(", len(*rows))\n")
	b.WriteString("\trefs := make([]relationRef, 0)\n")
	b.WriteString("\tkeys := make([]")
	b.WriteString(relation.RemotePKType)
	b.WriteString(", 0)\n")
	b.WriteString("\tseen := make(map[")
	b.WriteString(relation.RemotePKType)
	b.WriteString("]struct{})\n")
	b.WriteString("\tfor rowIndex := range *rows {\n")
	b.WriteString("\t\ttokens := []string{}\n")
	b.WriteString("\t\tif raw := string((*rows)[rowIndex].")
	b.WriteString(rowField)
	b.WriteString("); raw != \"\" { tokens = strings.Split(raw, \",\") }\n")
	b.WriteString("\t\tpayloads[rowIndex] = &")
	b.WriteString(relation.DTOName)
	b.WriteString("{\n")
	for _, field := range relation.PayloadFields {
		b.WriteString("\t\t\t")
		b.WriteString(field.GoName)
		b.WriteString(": make([]*")
		b.WriteString(field.GoType)
		b.WriteString(", len(tokens)),\n")
	}
	b.WriteString("\t\t}\n")
	b.WriteString("\t\tfor valueIndex, token := range tokens { key, ok := parseKey(token); if !ok { continue }; refs = append(refs, relationRef{rowIndex: rowIndex, valueIndex: valueIndex, key: key}); if _, exists := seen[key]; !exists { seen[key] = struct{}{}; keys = append(keys, key) } }\n")
	b.WriteString("\t}\n")
	b.WriteString("\tif len(keys) == 0 { for i := range *rows { (*rows)[i].")
	b.WriteString(relation.RelationGoField)
	b.WriteString(" = payloads[i] }; return nil }\n")
	b.WriteString("\trelated := make([]")
	b.WriteString(relation.RowDTOName)
	b.WriteString(", 0)\n")
	b.WriteString("\tif err := s.DBFor(ctx).Table(s.config.Database.Prefix + ")
	b.WriteString(strconv.Quote(relation.RemoteTable))
	b.WriteString(").Select(")
	for i, field := range relation.Fields {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(field.ColumnName))
	}
	b.WriteString(").Where(")
	b.WriteString(strconv.Quote(relation.RemotePK + " IN ?"))
	b.WriteString(", keys).Find(&related).Error; err != nil { return err }\n")
	b.WriteString("\tbyKey := make(map[")
	b.WriteString(relation.RemotePKType)
	b.WriteString("]*")
	b.WriteString(relation.RowDTOName)
	b.WriteString(", len(related))\n")
	b.WriteString("\tfor i := range related { byKey[related[i].")
	b.WriteString(relation.RemotePKGoField)
	b.WriteString("] = &related[i] }\n")
	b.WriteString("\tfor _, ref := range refs { if related, ok := byKey[ref.key]; ok {\n")
	for _, field := range relation.PayloadFields {
		b.WriteString("\t\t")
		b.WriteString("payloads[ref.rowIndex].")
		b.WriteString(field.GoName)
		b.WriteString("[ref.valueIndex] = ")
		if field.Nullable {
			b.WriteString("related.")
			b.WriteString(field.GoName)
		} else {
			b.WriteString("func() *")
			b.WriteString(field.GoType)
			b.WriteString(" { value := related.")
			b.WriteString(field.GoName)
			b.WriteString("; return &value }()")
		}
		b.WriteString("\n")
	}
	b.WriteString("\t} }\n")
	b.WriteString("\tfor i := range *rows { (*rows)[i].")
	b.WriteString(relation.RelationGoField)
	b.WriteString(" = payloads[i] }\n")
	b.WriteString("\treturn nil\n}\n\n")
	return b.String()
}

func renderRelationLoaderAggregator(modelData ModelData) string {
	var b strings.Builder
	b.WriteString("func (s *")
	b.WriteString(modelData.ClassName)
	b.WriteString("Model) loadRelations(ctx *gin.Context, rows *[]")
	b.WriteString(modelData.ClassName)
	b.WriteString(") error {\n")
	for _, relation := range modelData.Relations {
		b.WriteString("\tif err := s.load")
		b.WriteString(relation.RelationGoField)
		b.WriteString("Relations(ctx, rows); err != nil { return err }\n")
	}
	b.WriteString("\treturn nil\n}\n")
	return b.String()
}

func relationNameForField(fieldName string) string {
	relationName := fieldName
	if strings.HasSuffix(fieldName, "_ids") {
		relationName = strings.TrimSuffix(fieldName, "_ids")
	} else if strings.HasSuffix(fieldName, "_id") {
		relationName = strings.TrimSuffix(fieldName, "_id")
	} else {
		relationName += "_table"
	}
	return utils.SnakeToCamel(relationName, false)
}

// 解析模型方法（设置器、获取器等）
func parseModelMethods(field crudmodel.Field, modelData *ModelData) {
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
		if !slices.Contains(modelData.CityTextFields, field.Name) {
			modelData.CityTextFields = append(modelData.CityTextFields, field.Name)
		}
		modelData.Methods = append(modelData.Methods, assembleStub("mixins/model/getters/cityNames", map[string]string{
			"field":             fieldName + "Text",
			"originalFieldName": field.Name,
		}, false))
	}

}

func collectCityTextFields(fields []crudmodel.Field) []string {
	result := make([]string, 0)
	for _, field := range fields {
		if analyseField(field).DesignType == "city" && !slices.Contains(result, field.Name) {
			result = append(result, field.Name)
		}
	}
	return result
}

// 控制器/模型等文件的一些杂项属性解析
func parseSundryData(handlerData *HandlerData, indexVueData *IndexVueData, formVueData *FormVueData, field crudmodel.Field, table crudmodel.Table) {
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
	if (key == "show" || key == "operator" || key == "sortable") && (val == "0" || val == "1") {
		itemJson = " " + key + ": " + map[string]string{"0": "false", "1": "true"}[val] + ","
	} else if val == "false" || val == "true" {
		itemJson = " " + key + ": " + val + ","
	} else if key == "width" || key == "buttons" || translationCallRE.MatchString(val) {
		itemJson = " " + key + ": " + val + ","
	} else {
		itemJson = " " + key + ": " + strconv.Quote(val) + ","
	}
	return itemJson
}

var translationCallRE = regexp.MustCompile(`^t\(("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')\)$`)
var objectKeyRE = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func formatObjectKey(keyName string) string {
	if objectKeyRE.MatchString(keyName) {
		return keyName
	}
	return strconv.Quote(keyName)
}

func formatAttributeObjectKey(keyName string) string {
	if objectKeyRE.MatchString(keyName) {
		return keyName
	}
	return "'" + strings.ReplaceAll(keyName, "'", "\\'") + "'"
}

func getQuote(value string) string {
	if !strings.Contains(value, "'") {
		return "'"
	}
	return "\""
}

func getJsonFromArray(data map[string]string) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	jsonStr := ""
	for _, k := range keys {
		v := data[k]
		keyStr := " " + formatAttributeObjectKey(k) + ": "
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

func getJsonFromAny(data map[string]any) string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := data[key]
		var rendered string
		switch value := value.(type) {
		case string:
			rendered = strconv.Quote(value)
		case bool:
			rendered = strconv.FormatBool(value)
		case int:
			rendered = strconv.Itoa(value)
		case int8, int16, int32, int64:
			rendered = fmt.Sprint(value)
		case uint, uint8, uint16, uint32, uint64:
			rendered = fmt.Sprint(value)
		case float32:
			rendered = strconv.FormatFloat(float64(value), 'f', -1, 32)
		case float64:
			rendered = strconv.FormatFloat(value, 'f', -1, 64)
		case map[string]any:
			rendered = getJsonFromAny(value)
		default:
			rendered = fmt.Sprint(value)
		}
		parts = append(parts, " "+formatObjectKey(key)+": "+rendered)
	}
	if len(parts) == 0 {
		return "{}"
	}
	return "{" + strings.Join(parts, ",") + " }"
}

func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
