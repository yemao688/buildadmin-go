package crud_helper

import (
	"bytes"
	"fmt"
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/utils"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
	"gorm.io/gen"
	"gorm.io/gorm"
)

// writeModelFiles 写入共享贫血实体（internal/model）与 admin 仓库
// （internal/admin/repository，XxxRepository）；DTO 由 writeHandlerFile
// 另行落盘。返回实体 struct 内容供 DTO/测试复用。
func writeModelFiles(db *gorm.DB, tablePk string, fullTableName string, tableName string, modelData ModelData, entityFile, repositoryFile NameInfo) (string, error) {
	if tablePk != "" {
		modelData.Pk = tablePk
	}
	structContent, err := getGenerateStruct(db, fullTableName, modelData.ClassName, modelData.ModelFieldType)
	if err != nil {
		return "", err
	}
	modelData.StructTemp = addCityTextFields(structContent, modelData.CityTextFields)
	modelData.StructTemp = addRelationFields(modelData.StructTemp, modelData.RelationFields)
	prepareModelTimestampData(&modelData)
	applyBaseRepositoryPackageRef(&modelData, repositoryFile)

	// 实体文件（扁平 internal/model/<entity>.go，包名 model）
	entityContent, err := render(entityFile.ParseFile, entityTemp, modelData)
	if err != nil {
		return "", err
	}
	if err := writeGoFile(entityFile.ParseFile, entityContent); err != nil {
		return "", err
	}

	// 仓库文件（internal/admin/repository/<path>.go，XxxRepository）
	repositoryContent, err := render(repositoryFile.ParseFile, modelTemp, modelData)
	if err != nil {
		return "", err
	}
	if err := writeGoFile(repositoryFile.ParseFile, repositoryContent); err != nil {
		return "", err
	}

	// 扁平仓库包是 wire 静态聚合根包：并入合并 ProviderSet，不动 wire.go。
	if err := writeProvider(repositoryFile.RootFileName, modelData.ClassName+"Repository"); err != nil {
		return "", err
	}
	return structContent, nil
}

// applyBaseRepositoryPackageRef 为子包仓库设置根仓库包（internal/admin/repository，
// 提供 QueryBuilder）的限定引用；根仓库包保持空值，输出与既有写法一致。
func applyBaseRepositoryPackageRef(modelData *ModelData, repositoryFile NameInfo) {
	root := filepath.ToSlash(repositoryFile.RootFileName)
	if root == "internal/admin/repository" {
		return
	}
	if !strings.HasPrefix(root, "internal/admin/repository/") {
		return
	}
	modelData.BaseModelImport = "buildadmin-go/internal/admin/repository"
	modelData.BaseModelAlias = "adminmodel"
	modelData.BaseModelQualifier = modelData.BaseModelAlias + "."
}

func addCityTextFields(structContent string, cityFields []string) string {
	if len(cityFields) == 0 {
		return structContent
	}
	index := strings.LastIndex(structContent, "}")
	if index < 0 {
		return structContent
	}
	var fields strings.Builder
	for _, field := range cityFields {
		goName := utils.SnakeToCamel(field+"_text", true)
		fields.WriteString("\t" + goName + " string `json:\"" + field + "_text\" gorm:\"-\"`\n")
	}
	return structContent[:index] + fields.String() + structContent[index:]
}

func addRelationFields(structContent, relationFields string) string {
	if relationFields == "" {
		return structContent
	}
	index := strings.LastIndex(structContent, "}")
	if index < 0 {
		return structContent
	}
	return structContent[:index] + relationFields + structContent[index:]
}

func getGenerateStruct(db *gorm.DB, fullTableName string, structName string, fieldTypeOverrides map[string]string) (string, error) {
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
	g.UseDB(db)
	keys := make([]string, 0, len(fieldTypeOverrides))
	for columnName := range fieldTypeOverrides {
		keys = append(keys, columnName)
	}
	sort.Strings(keys)
	options := make([]gen.ModelOpt, 0, len(keys))
	for _, columnName := range keys {
		options = append(options, gen.FieldType(columnName, fieldTypeOverrides[columnName]))
	}
	data := g.GenerateModelAs(fullTableName, structName, options...)

	var buf bytes.Buffer
	tpl, err := template.New(StructTmpl).Parse(StructTmpl)
	if err != nil {
		return "", err
	}
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return normalizeGeneratedIDInitialisms(buf.String()), nil
}

var generatedIDFieldRE = regexp.MustCompile(`(?m)^(\s*)([A-Za-z][A-Za-z0-9]*)Ids(\s+)`)

func normalizeGeneratedIDInitialisms(structContent string) string {
	return generatedIDFieldRE.ReplaceAllString(structContent, `${1}${2}IDs${3}`)
}

// func buildModelAppend() {

// }

// func buildFormatSimpleArray(data []string, tab int) string {
// 	if len(data) == 0 {
// 		return "[]"
// 	}
// 	str := "["
// 	for _, v := range data {
// 		_, err := strconv.Atoi(v)
// 		if v == "undefined" || v == "false" || err != nil {
// 			str += Tab(tab) + v + ","
// 		} else {
// 			quote := getQuote(v)
// 			str += Tab(tab) + quote + v + quote + ", "
// 		}
// 	}
// 	return str + Tab(tab-1) + "]"
// }

// func buildModelFieldType() {

// }

// applyBaseHandlerPackageRef 为子包 handler/registrar 设置根包 Base 与包级函数的
// 限定引用；根包（internal/admin/handler）保持空值，生成输出与历史一致。
func applyBaseHandlerPackageRef(qualifier *string, alias *string, importPath *string, handlerFile NameInfo) {
	root := filepath.ToSlash(handlerFile.RootFileName)
	if root == "internal/admin/handler" {
		return
	}
	if !strings.HasPrefix(root, "internal/admin/handler/") {
		return
	}
	*alias = "adminhandler"
	*importPath = "buildadmin-go/internal/admin/handler"
	*qualifier = *alias + "."
}

func writeHandlerFile(handlerData HandlerData, handlerFile NameInfo, structContent string, dtoFile, registrarFile NameInfo) error {
	applyBaseHandlerPackageRef(&handlerData.BaseHandlerQualifier, &handlerData.BaseHandlerAlias, &handlerData.BaseHandlerImport, handlerFile)

	// 请求参数结构体落盘到独立 DTO 文件（internal/admin/dto/<table>.go）
	paramStruct := buildParamStruct(structContent, handlerData)
	dtoContent, err := render(dtoFile.ParseFile, dtoTemp, struct {
		Namespace     string
		ValidateParam string
	}{Namespace: dtoFile.Namespace, ValidateParam: paramStruct})
	if err != nil {
		return err
	}
	if err := writeGoFile(dtoFile.ParseFile, dtoContent); err != nil {
		return err
	}

	//渲染文件内容
	handlerContent, err := render(handlerFile.ParseFile, handlerTemp, handlerData)
	if err != nil {
		return err
	}
	//写入文件
	if err := writeGoFile(handlerFile.ParseFile, handlerContent); err != nil {
		return err
	}
	// 扁平 handler 包是 wire 静态聚合根包：并入合并 ProviderSet，不动 wire.go。
	if err := writeProvider(handlerFile.RootFileName, handlerData.ClassName+"Handler"); err != nil {
		return err
	}
	// 路由注册器落 internal/admin/router/<table>.go，并接入两个锚点：
	// 该包合并 ProviderSet 与 ProvideRegistrars（新模块一行 + handler 参数）。
	if err := writeRegistrarFile(handlerData, registrarFile); err != nil {
		return err
	}
	if err := writeRouterProviderEntry(handlerData.ClassName); err != nil {
		return err
	}
	if err := writeAdminRouterEntry(handlerData.ClassName); err != nil {
		return err
	}
	if handlerData.RegisterAtomicRoute != nil {
		name := handlerData.RouteName
		if name == "" {
			name = strings.ToLower(handlerData.ClassName[:1]) + handlerData.ClassName[1:]
		}
		handlerData.RegisterAtomicRoute("POST", name+"/add")
		handlerData.RegisterAtomicRoute("POST", name+"/edit")
		handlerData.RegisterAtomicRoute("DELETE", name+"/del")
	}
	return nil
}

// buildParamStruct 由实体 struct 内容派生请求 DTO 的 struct 声明
// （"type XxxParam struct {...}"），去掉 gorm tag 并应用参数类型改写与排除字段。
func buildParamStruct(structContent string, handlerData HandlerData) string {
	index := strings.Index(structContent, "struct {")
	if index == -1 {
		return ""
	}
	validateContent := "type " + handlerData.ClassName + "Param " + structContent[index:]
	re := regexp.MustCompile(`gorm:"[^"]*" `)
	validateContent = re.ReplaceAllString(validateContent, "")
	validateContent = rewriteFlexNumericParamFields(validateContent, handlerData.ParamTypeOverrides)
	return excludeParamFieldsWithPrimaryKey(validateContent, handlerData.ExcludeParamFields, handlerData.PkJSONName)
}

// renderDTO 渲染 DTO 文件内容（供测试与生产共用）。
func renderDTO(paramStruct string) (string, error) {
	return render("", dtoTemp, struct {
		Namespace     string
		ValidateParam string
	}{Namespace: "dto", ValidateParam: paramStruct})
}

func excludeParamFields(validateContent string, exclude []string) string {
	return excludeParamFieldsWithPrimaryKey(validateContent, exclude, "id")
}

func excludeParamFieldsWithPrimaryKey(validateContent string, exclude []string, primaryKey string) string {
	var newLines []string
	lines := strings.Split(validateContent, "\n")
	excludedFields := []string{primaryKey}
	excludedFields = append(excludedFields, exclude...)
	for _, line := range lines {
		skip := false
		for _, f := range excludedFields {
			if f != "" && strings.Contains(line, `json:"`+f+`"`) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		newLines = append(newLines, line)
	}
	return strings.Join(newLines, "\n")
}

// renderModel renders the repository template to a string for tests.
func renderModel(modelData ModelData) (string, error) {
	if modelData.PkGoType == "" {
		modelData.PkGoType = "int32"
	}
	if modelData.DataScopeOwnerGoField != "" && modelData.DataScopeOwnerGoType == "" {
		modelData.DataScopeOwnerGoType = "int32"
	}
	if len(modelData.Relations) > 0 && modelData.RelationStructs == "" {
		finalizeRelationMetadata(&modelData)
	}
	modelData.StructTemp = addRelationFields(modelData.StructTemp, modelData.RelationFields)
	prepareModelTimestampData(&modelData)
	return render("", modelTemp, modelData)
}

// renderEntity renders the shared entity file content to a string for tests.
func renderEntity(modelData ModelData) (string, error) {
	if len(modelData.Relations) > 0 && modelData.RelationStructs == "" {
		finalizeRelationMetadata(&modelData)
	}
	modelData.StructTemp = addRelationFields(modelData.StructTemp, modelData.RelationFields)
	prepareModelTimestampData(&modelData)
	return render("", entityTemp, modelData)
}

func prepareModelTimestampData(modelData *ModelData) {
	modelData.HasCreateTime = false
	modelData.HasUpdateTime = false
	modelData.CreateTime = ""
	modelData.UpdateTime = ""
	updateJSON := ""
	// Detect canonical auto timestamps from generated struct field + json tag,
	// independent of Go type (plain int64 after override skip, or any integer form).
	fieldRe := regexp.MustCompile(`(?m)^\s*([A-Za-z_][A-Za-z0-9_]*)\s+\S+.*json:"([^"]+)"`)
	for _, match := range fieldRe.FindAllStringSubmatch(modelData.StructTemp, -1) {
		goName := match[1]
		jsonName := strings.Split(match[2], ",")[0]
		switch strings.ToLower(jsonName) {
		case "create_time", "createtime":
			modelData.HasCreateTime = true
			modelData.CreateTime = goName
		case "update_time", "updatetime":
			modelData.HasUpdateTime = true
			modelData.UpdateTime = goName
			updateJSON = jsonName
		}
	}
	modelData.HasWeigh = regexp.MustCompile(`(?m)^\s*Weigh\s+int32\s+`).MatchString(modelData.StructTemp)
	if modelData.HasUpdateTime && updateJSON != "" && !slices.Contains(modelData.EditableColumns, updateJSON) {
		// Keep update column writable so automatic UpdateTime assignment is persisted.
		modelData.EditableColumns = append(modelData.EditableColumns, updateJSON)
		modelData.EditableColumnsGo = joinQuotedColumns(modelData.EditableColumns)
	}
}

// renderHandler renders the handler template to a string for tests.
func renderHandler(handlerData HandlerData) (string, error) {
	if handlerData.PkJSONName == "" {
		handlerData.PkJSONName = "id"
	}
	return render("", handlerTemp, handlerData)
}

func rewriteFlexNumericParamFields(content string, overrides map[string]string) string {
	re := regexp.MustCompile("(?m)^(\\s*[A-Za-z]\\w*\\s+)([^\\s]+)(\\s+.*json:\"([^\"]+)\")")
	return re.ReplaceAllStringFunc(content, func(line string) string {
		matches := re.FindStringSubmatch(line)
		jsonName := strings.Split(matches[4], ",")[0]
		typeName := overrides[jsonName]
		if typeName == "" {
			typeName = map[string]string{
				"bool":    "validate.FlexBool",
				"int32":   "validate.FlexInt32",
				"int64":   "validate.FlexInt64",
				"float64": "validate.FlexFloat64",
			}[matches[2]]
		}
		if typeName == "" {
			return line
		}
		return matches[1] + typeName + matches[3] + line[len(matches[0]):]
	})
}

func render(file string, temp string, data any) (string, error) {
	var buf bytes.Buffer
	tpl, err := template.New(temp).Parse(temp)
	if err != nil {
		return "", err
	}
	if err := tpl.Execute(&buf, data); err != nil {
		return "", err
	}
	text, err := imports.Process(file, buf.Bytes(), nil)
	if err != nil {
		return "", err
	}
	return string(text), nil
}

// writeRegistrarFile 写入 admin 渠道路由注册器（internal/admin/router/<table>.go）。
// 注册器是 router 包自身成员：CRUDRoutes/CRUDCapabilities 同包引用，仅 handler
// 类型需要包限定。
func writeRegistrarFile(handlerData HandlerData, registrarFile NameInfo) error {
	registrarPath := registrarFile.ParseFile
	data := RegistrarData{
		Namespace:            registrarFile.Namespace,
		ClassName:            handlerData.ClassName,
		RouteName:            lowerFirst(handlerData.ClassName),
		RoutePath:            handlerData.RouteName,
		BaseHandlerQualifier: "handler.",
		BaseHandlerAlias:     "handler",
		BaseHandlerImport:    "buildadmin-go/internal/admin/handler",
	}
	content, err := render(registrarPath, registrarTemp, data)
	if err != nil {
		return err
	}
	return writeGoFile(registrarPath, content)
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	return strings.ToLower(value[:1]) + value[1:]
}

func writeRegistrarProviderEntry(name string, handlerRoot string) error {
	path := filepath.Join(utils.RootPath(), "internal", "router", "registrar_set.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, err := addRegistrarProviderEntry(string(content), name, handlerRoot)
	if err != nil {
		return err
	}
	return writeGoFile(path, updated)
}

// handlerImportRef 返回 handler 目录在共享文件中使用的 import 路径、首选别名
// 以及是否为子包（根 handler 包在 registrar_set.go 中固定使用别名 admin）。
func handlerImportRef(handlerRoot string) (importPath string, alias string, isSub bool) {
	root := filepath.ToSlash(handlerRoot)
	importPath = "buildadmin-go/" + root
	if root == "internal/admin/handler" {
		return importPath, "admin", false
	}
	return importPath, path.Base(root), true
}

var importLinePattern = regexp.MustCompile(`(?m)^\t(?:(\w+) )?"([^"]+)"$`)

// collectImports 解析 import 块中已有的别名与路径映射。
func collectImports(content string) (aliasToPath map[string]string, pathToAlias map[string]string) {
	aliasToPath = map[string]string{}
	pathToAlias = map[string]string{}
	for _, matches := range importLinePattern.FindAllStringSubmatch(content, -1) {
		alias, importPath := matches[1], matches[2]
		if alias == "" {
			alias = path.Base(importPath)
		}
		aliasToPath[alias] = importPath
		pathToAlias[importPath] = alias
	}
	return aliasToPath, pathToAlias
}

// resolveImportAlias 为 importPath 选择文件内稳定的别名：已导入则复用，
// 首选别名被其它路径占用时回退为 fallback。
func resolveImportAlias(content, importPath, preferred, fallback string) (alias string, imported bool) {
	aliasToPath, pathToAlias := collectImports(content)
	if alias, ok := pathToAlias[importPath]; ok {
		return alias, true
	}
	if _, taken := aliasToPath[preferred]; taken {
		return fallback, false
	}
	return preferred, false
}

func ensureImportLine(content, alias, importPath string) string {
	if _, imported := resolveImportAlias(content, importPath, alias, alias); imported {
		return content
	}
	marker := "import (\n"
	index := strings.Index(content, marker)
	if index < 0 {
		return content
	}
	insertAt := index + len(marker)
	return content[:insertAt] + "\t" + alias + " \"" + importPath + "\"\n" + content[insertAt:]
}

// removeImportLineIfUnused 在别名不再被引用时移除对应 import 行。
func removeImportLineIfUnused(content, alias, importPath string) string {
	if strings.Contains(content, alias+".") {
		return content
	}
	line := "\t" + alias + " \"" + importPath + "\"\n"
	return strings.Replace(content, line, "", 1)
}

// registrarVarCandidates 返回参数变量名候选：子包加包名前缀避免跨包同类名冲突；
// 兼容历史上小写类名的裸变量名写法。
func registrarVarCandidates(name string, handlerRoot string) []string {
	_, alias, isSub := handlerImportRef(handlerRoot)
	candidates := []string{lowerFirst(name) + "Registrar", lowerFirst(name)}
	if isSub {
		candidates = append([]string{lowerFirst(alias) + name + "Registrar"}, candidates...)
	}
	return candidates
}

func addRegistrarProviderEntry(content, name string, handlerRoot string) (string, error) {
	registrarType := name + "Registrar"
	importPath, preferred, isSub := handlerImportRef(handlerRoot)
	alias, _ := resolveImportAlias(content, importPath, preferred, "admin"+utils.SnakeToCamel(preferred, true))
	registrarVar := registrarVarCandidates(name, handlerRoot)[0]
	param := "\t" + registrarVar + " *" + alias + "." + registrarType + ",\n"
	entry := "\t\t" + registrarVar + ",\n"

	if isSub {
		content = ensureImportLine(content, alias, importPath)
	}
	if !strings.Contains(content, " *"+alias+"."+registrarType+",\n") {
		marker := ") []RouteRegistrar {"
		index := strings.Index(content, marker)
		if index < 0 {
			return "", fmt.Errorf("registrar provider signature anchor not found")
		}
		content = content[:index] + param + content[index:]
	}
	if !strings.Contains(content, entry) {
		marker := "\n\t}\n}"
		index := strings.LastIndex(content, marker)
		if index < 0 {
			return "", fmt.Errorf("registrar provider return anchor not found")
		}
		insertAt := index + 1
		content = content[:insertAt] + entry + content[insertAt:]
	}
	return content, nil
}

func RemoveRegistrarProvider(name string, handlerRoot string) error {
	path := filepath.Join(utils.RootPath(), "internal", "router", "registrar_set.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, err := removeRegistrarProviderEntry(string(content), name, handlerRoot)
	if err != nil {
		return err
	}
	return writeGoFile(path, updated)
}

// removeRegistrarProviderEntry 按整行移除 registrar 参数与返回值条目。
// 类型按包限定符精确匹配（*order.UserRegistrar 与 *admin.UserRegistrar 是不同模块），
// 返回条目只移除实际命中参数的变量；子包 import 在无引用时一并移除。
func removeRegistrarProviderEntry(content, name string, handlerRoot string) (string, error) {
	registrarType := name + "Registrar"
	importPath, preferred, isSub := handlerImportRef(handlerRoot)
	alias, _ := resolveImportAlias(content, importPath, preferred, preferred)
	qualifiedType := alias + "." + registrarType
	varCandidates := registrarVarCandidates(name, handlerRoot)
	isVar := func(value string) bool {
		return slices.Contains(varCandidates, value)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", err
	}
	var removals []sourceRemoval
	matchedVars := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Name == nil || function.Name.Name != "ProvideRegistrars" || function.Type.Params == nil {
			return true
		}
		matchField := func(field *ast.Field) bool {
			return len(field.Names) == 1 && isVar(field.Names[0].Name) && strings.Contains(formatNodeText(fset, field, content), qualifiedType)
		}
		params := make([]ast.Node, 0, len(function.Type.Params.List))
		removed := false
		for _, field := range function.Type.Params.List {
			if matchField(field) {
				matchedVars[field.Names[0].Name] = true
				removed = true
				continue
			}
			params = append(params, field)
		}
		if removed {
			removals = append(removals, removeListElementRanges(content, fset, function.Type.Params.Opening, function.Type.Params.Closing, fieldsAsNodes(function.Type.Params.List), func(node ast.Node) bool {
				field, ok := node.(*ast.Field)
				return ok && matchField(field)
			})...)
		}
		return true
	})
	ast.Inspect(file, func(node ast.Node) bool {
		composite, ok := node.(*ast.CompositeLit)
		if !ok || len(composite.Elts) == 0 {
			return true
		}
		removals = append(removals, removeListElementRanges(content, fset, composite.Lbrace, composite.Rbrace, exprsAsNodes(composite.Elts), func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			return ok && matchedVars[ident.Name]
		})...)
		return true
	})
	if len(removals) == 0 {
		return content, nil
	}
	content = applySourceRemovals(content, removals)
	if isSub {
		_, pathToAlias := collectImports(content)
		if actual, ok := pathToAlias[importPath]; ok {
			content = removeImportLineIfUnused(content, actual, importPath)
		}
	}
	formatted, err := formatGoCode(content)
	if err != nil {
		return "", err
	}
	return canonicalizeGoContent(formatted), nil
}

// wireProviderSetRef 计算子包在 cmd/server/wire.go 中的 ProviderSet 引用。
// 根包（internal/admin/handler、internal/admin/repository、internal/api/handler）
// 已在 wire.Build 静态聚合，无需处理；拍平后生成器不再追加/移除任何 ProviderSet，
// 本函数仅被历史嵌套布局删除兼容路径调用。
func wireProviderSetRef(rootDir string) (importPath, alias, anchor string, needed bool, err error) {
	root := filepath.ToSlash(rootDir)
	wiredRoots := map[string]string{
		"internal/admin/handler":    "adminHandler",
		"internal/admin/repository": "adminRepo",
		"internal/api/handler":      "apiHandler",
	}
	if _, ok := wiredRoots[root]; ok {
		return "", "", "", false, nil
	}
	segments := strings.Split(root, "/")
	anchorAlias := ""
	subStart := -1
	for i := len(segments) - 1; i > 0; i-- {
		if parentAlias, ok := wiredRoots[strings.Join(segments[:i], "/")]; ok {
			anchorAlias = parentAlias
			subStart = i
			break
		}
	}
	if anchorAlias == "" {
		return "", "", "", false, fmt.Errorf("no wired root provider set found for %q", rootDir)
	}
	sub := make([]string, 0, len(segments)-subStart)
	for _, segment := range segments[subStart:] {
		sub = append(sub, utils.SnakeToCamel(segment, true))
	}
	anchorParent := path.Base(strings.Join(segments[:subStart], "/"))
	suffix := map[string]string{"handler": "Handler", "repository": "Repo", "model": "Model"}[anchorParent]
	if suffix == "" {
		suffix = utils.SnakeToCamel(anchorParent, true)
	}
	alias = lowerFirst(strings.Join(sub, "")) + suffix
	return "buildadmin-go/" + root, alias, "\t\t" + anchorAlias + ".ProviderSet,\n", true, nil
}

// AddWireProviderSet 将子包 ProviderSet 聚合并入 cmd/server/wire.go（幂等）。
func AddWireProviderSet(rootDir string) error {
	importPath, alias, anchor, needed, err := wireProviderSetRef(rootDir)
	if err != nil || !needed {
		return err
	}
	wirePath := filepath.Join(utils.RootPath(), "cmd", "server", "wire.go")
	content, err := os.ReadFile(wirePath)
	if err != nil {
		return err
	}
	updated, err := addWireProviderSetEntry(string(content), importPath, alias, anchor)
	if err != nil {
		return err
	}
	return writeGoFile(wirePath, updated)
}

func addWireProviderSetEntry(content, importPath, alias, anchor string) (string, error) {
	setLine := "\t\t" + alias + ".ProviderSet,\n"
	if strings.Contains(content, setLine) {
		return content, nil
	}
	aliasToPath, _ := collectImports(content)
	if existing, taken := aliasToPath[alias]; taken && existing != importPath {
		return "", fmt.Errorf("wire.go import alias %q already bound to %q, cannot add %q", alias, existing, importPath)
	}
	content = ensureImportLine(content, alias, importPath)
	index := strings.Index(content, anchor)
	if index < 0 {
		return "", fmt.Errorf("wire.go provider set anchor %q not found", strings.TrimSpace(anchor))
	}
	insertAt := index + len(anchor)
	return content[:insertAt] + setLine + content[insertAt:], nil
}

// RemoveWireProviderSet 从 cmd/server/wire.go 移除子包 ProviderSet 聚合（幂等）。
// 仅用于历史嵌套布局删除兼容；拍平根包与未知根一律不处理（历史 manifest 的
// wire.go 清理是尽力而为——当前 wire.go 由 wire 重生成，旧条目自然消失）。
func RemoveWireProviderSet(rootDir string) error {
	importPath, alias, _, needed, err := wireProviderSetRef(rootDir)
	if err != nil || !needed {
		return nil
	}
	providerPath := filepath.Join(utils.RootPath(), rootDir, "provider.go")
	if content, err := os.ReadFile(providerPath); err == nil {
		remaining, err := countWireProviderSetEntries(string(content))
		if err != nil {
			return err
		}
		if remaining > 0 {
			// 多模块共享包里仍有其他模块由该 ProviderSet 提供，不能提前从 wire.go 摘掉整包引用。
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	wirePath := filepath.Join(utils.RootPath(), "cmd", "server", "wire.go")
	content, err := os.ReadFile(wirePath)
	if err != nil {
		return err
	}
	updated := removeWireProviderSetEntry(string(content), importPath, alias)
	return writeGoFile(wirePath, updated)
}

func removeWireProviderSetEntry(content, importPath, alias string) string {
	_, pathToAlias := collectImports(content)
	if actual, ok := pathToAlias[importPath]; ok {
		alias = actual
	}
	content = strings.Replace(content, "\t\t"+alias+".ProviderSet,\n", "", 1)
	return removeImportLineIfUnused(content, alias, importPath)
}

func countWireProviderSetEntries(content string) (int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return 0, err
	}
	entries := 0
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "ProviderSet" || len(spec.Values) != 1 {
			return true
		}
		call, ok := spec.Values[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != "NewSet" {
			return true
		}
		entries = len(call.Args)
		return false
	})
	return entries, nil
}

// adminRouterProviderPath 是 admin 渠道路由注册器的中心锚点文件：
// ProviderSet（NewXxxRegistrar 列表）与 ProvideRegistrars（每模块一行 + handler 参数）。
func adminRouterProviderPath() string {
	return filepath.Join(utils.RootPath(), "internal", "admin", "router", "provider.go")
}

// writeAdminRouterEntry 向 internal/admin/router/provider.go 的 ProvideRegistrars
// 增加一行 NewXxxRegistrar(xxx) 与对应 handler 参数（幂等）。ProviderSet 列表由
// writeProvider 另行追加。
func writeAdminRouterEntry(className string) error {
	path := adminRouterProviderPath()
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, err := addProvideRegistrarsEntry(string(content), className)
	if err != nil {
		return err
	}
	return writeGoFile(path, updated)
}

func addProvideRegistrarsEntry(content, className string) (string, error) {
	if strings.Contains(content, "New"+className+"Registrar(") {
		return content, nil
	}
	varName := lowerFirst(className)
	param := "\t" + varName + " *handler." + className + "Handler,\n"
	entry := "\t\tNew" + className + "Registrar(" + varName + "),\n"

	// handler 参数：插到 ProvideRegistrars 参数列表结束前
	paramMarker := ") []Registrar {"
	paramIndex := strings.Index(content, paramMarker)
	if paramIndex < 0 {
		return "", fmt.Errorf("ProvideRegistrars signature anchor not found")
	}
	content = content[:paramIndex] + param + content[paramIndex:]

	// 返回 slice 条目：插到 return 列表结束前（文件末 ProvideRegistrars 的收尾）
	entryMarker := "\t}\n}"
	entryIndex := strings.LastIndex(content, entryMarker)
	if entryIndex < 0 {
		return "", fmt.Errorf("ProvideRegistrars return anchor not found")
	}
	content = content[:entryIndex] + entry + content[entryIndex:]
	return content, nil
}

// removeAdminRouterEntry 从 internal/admin/router/provider.go 精确移除模块：
// ProvideRegistrars 的 handler 参数与返回条目，以及 ProviderSet 的 NewXxxRegistrar。
func removeAdminRouterEntry(className string) error {
	path := adminRouterProviderPath()
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, err := removeProvideRegistrarsEntry(string(content), className)
	if err != nil {
		return err
	}
	return writeGoFile(path, updated)
}

// removeProvideRegistrarsEntry 从 ProvideRegistrars 精确移除模块的 handler 参数
// 与返回 slice 条目；同时移除 ProviderSet 中的 NewXxxRegistrar 条目（幂等）。
func removeProvideRegistrarsEntry(content, className string) (string, error) {
	varName := lowerFirst(className)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", err
	}
	var removals []sourceRemoval
	// 1. ProvideRegistrars 参数
	ast.Inspect(file, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Name == nil || function.Name.Name != "ProvideRegistrars" || function.Type.Params == nil {
			return true
		}
		matchField := func(field *ast.Field) bool {
			return len(field.Names) == 1 && field.Names[0].Name == varName && strings.Contains(formatNodeText(fset, field, content), className+"Handler")
		}
		removed := false
		for _, field := range function.Type.Params.List {
			if matchField(field) {
				removed = true
				break
			}
		}
		if removed {
			removals = append(removals, removeListElementRanges(content, fset, function.Type.Params.Opening, function.Type.Params.Closing, fieldsAsNodes(function.Type.Params.List), func(node ast.Node) bool {
				field, ok := node.(*ast.Field)
				return ok && matchField(field)
			})...)
		}
		return true
	})
	// 2. ProvideRegistrars 返回 slice 中的 NewXxxRegistrar(xxx) 调用
	ast.Inspect(file, func(node ast.Node) bool {
		composite, ok := node.(*ast.CompositeLit)
		if !ok || len(composite.Elts) == 0 {
			return true
		}
		removals = append(removals, removeListElementRanges(content, fset, composite.Lbrace, composite.Rbrace, exprsAsNodes(composite.Elts), func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return false
			}
			ident, ok := call.Fun.(*ast.Ident)
			return ok && ident.Name == "New"+className+"Registrar"
		})...)
		return true
	})
	// 3. ProviderSet 中的 NewXxxRegistrar 条目（wire.NewSet 参数）
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != "NewSet" {
			return true
		}
		removals = append(removals, removeListElementRanges(content, fset, call.Lparen, call.Rparen, exprsAsNodes(call.Args), func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			return ok && ident.Name == "New"+className+"Registrar"
		})...)
		return true
	})
	if len(removals) == 0 {
		return content, nil
	}
	content = applySourceRemovals(content, removals)
	formatted, err := formatGoCode(content)
	if err != nil {
		return "", err
	}
	return canonicalizeGoContent(formatted), nil
}

// writeRouterProviderEntry 向 internal/admin/router/provider.go 的
// var ProviderSet = wire.NewSet(...) 追加 NewXxxRegistrar（幂等）。
// 该文件还包含 ProvideRegistrars 等函数声明，不能使用 writeProvider 的
// 行尾启发式。
func writeRouterProviderEntry(className string) error {
	path := adminRouterProviderPath()
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, err := addProviderSetEntry(string(content), className+"Registrar")
	if err != nil {
		return err
	}
	return writeGoFile(path, updated)
}

// addProviderSetEntry 向文件内第一个 var ProviderSet = wire.NewSet(...)
// 追加 New<name> 条目（幂等），支持文件同时包含函数声明的场景。
func addProviderSetEntry(content, name string) (string, error) {
	target := "New" + name
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", err
	}
	var setCall *ast.CallExpr
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || spec.Names[0].Name != "ProviderSet" || len(spec.Values) != 1 {
			return true
		}
		call, ok := spec.Values[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		setCall = call
		return false
	})
	if setCall == nil {
		return "", fmt.Errorf("ProviderSet declaration not found")
	}
	for _, arg := range setCall.Args {
		if ident, ok := arg.(*ast.Ident); ok && ident.Name == target {
			return content, nil // 幂等
		}
	}
	insertAt := fset.PositionFor(setCall.Rparen, false).Offset
	return content[:insertAt] + "\t" + target + ",\n" + content[insertAt:], nil
}

func writeProvider(dir string, name string) error {
	providerPath := filepath.Join(utils.RootPath(), dir, "provider.go")
	if err := ValidateGeneratedAbsolutePath(providerPath, "internal/admin/router", "internal/admin/repository", "internal/admin/handler", "internal/admin/model", "internal/common/model"); err != nil {
		return err
	}
	content, err := os.ReadFile(providerPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		pkg := filepath.Base(dir)
		content = []byte("package " + pkg + "\n\nimport \"github.com/google/wire\"\n\nvar ProviderSet = wire.NewSet()\n")
		if err := writeGoFile(providerPath, string(content)); err != nil {
			return err
		}
	}
	//判断是否已经生成过
	if strings.Contains(string(content), "New"+name) {
		return nil
	}

	lastIndex := strings.LastIndex(string(content), ")")
	head := strings.TrimRight(string(content)[:lastIndex], " \t\r\n")
	separator := ""
	if !strings.HasSuffix(head, "(") && !strings.HasSuffix(head, ",") {
		// gofmt 会把单参数 NewSet(arg) 折叠成单行，追加第二个条目时必须补逗号
		separator = ","
	}
	content = []byte(head + separator + "\n\tNew" + name + ",\n)")
	return writeGoFile(providerPath, string(content))
}

// 移除生成的相应代码
func RemoveProvider(dir string, name string) error {
	content, err := os.ReadFile(filepath.Join(utils.RootPath(), dir, "provider.go"))
	if err != nil {
		return err
	}
	newContent, err := removeProviderEntry(string(content), name)
	if err != nil {
		return err
	}
	return writeGoFile(filepath.Join(utils.RootPath(), dir, "provider.go"), newContent)
}

// removeProviderEntry 按整行移除 provider 条目（gofmt 折叠残留空行）。
func removeProviderEntry(content, name string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", err
	}
	var removals []sourceRemoval
	target := "New" + name
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != "NewSet" {
			return true
		}
		removals = append(removals, removeListElementRanges(content, fset, call.Lparen, call.Rparen, exprsAsNodes(call.Args), func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			return ok && ident.Name == target
		})...)
		return true
	})
	if len(removals) > 0 {
		content = applySourceRemovals(content, removals)
	}
	newContent, err := formatGoCode(content)
	if err != nil {
		return "", err
	}
	return canonicalizeGoContent(newContent), nil
}

type sourceRemoval struct {
	start int
	end   int
}

func fieldsAsNodes(fields []*ast.Field) []ast.Node {
	nodes := make([]ast.Node, len(fields))
	for index, field := range fields {
		nodes[index] = field
	}
	return nodes
}

func exprsAsNodes(exprs []ast.Expr) []ast.Node {
	nodes := make([]ast.Node, len(exprs))
	for index, expr := range exprs {
		nodes[index] = expr
	}
	return nodes
}

func removeListElementRanges(content string, fset *token.FileSet, opening, closing token.Pos, elements []ast.Node, match func(ast.Node) bool) []sourceRemoval {
	var removals []sourceRemoval
	openOffset := fset.PositionFor(opening, false).Offset
	closeOffset := fset.PositionFor(closing, false).Offset
	multiline := strings.Contains(content[openOffset:closeOffset], "\n")
	for index, element := range elements {
		if !match(element) {
			continue
		}
		start := fset.PositionFor(element.Pos(), false).Offset
		end := fset.PositionFor(element.End(), false).Offset
		if index < len(elements)-1 {
			end = fset.PositionFor(elements[index+1].Pos(), false).Offset
		} else {
			end = consumeListComma(content, end)
			if multiline {
				start = listElementLineStart(content, start)
			} else if index > 0 {
				start = fset.PositionFor(elements[index-1].End(), false).Offset
			}
		}
		removals = append(removals, sourceRemoval{start: start, end: end})
	}
	return removals
}

func consumeListComma(content string, offset int) int {
	for offset < len(content) && (content[offset] == ' ' || content[offset] == '\t') {
		offset++
	}
	if offset < len(content) && content[offset] == ',' {
		offset++
		if offset < len(content) && content[offset] == '\r' {
			offset++
		}
		if offset < len(content) && content[offset] == '\n' {
			offset++
		}
	}
	return offset
}

func listElementLineStart(content string, offset int) int {
	for offset > 0 && content[offset-1] != '\n' {
		offset--
	}
	return offset
}

func applySourceRemovals(content string, removals []sourceRemoval) string {
	sort.Slice(removals, func(i, j int) bool {
		if removals[i].start == removals[j].start {
			return removals[i].end > removals[j].end
		}
		return removals[i].start < removals[j].start
	})
	merged := removals[:0]
	for _, removal := range removals {
		if removal.start < 0 || removal.end <= removal.start || removal.end > len(content) {
			continue
		}
		if len(merged) > 0 && removal.start <= merged[len(merged)-1].end {
			if removal.end > merged[len(merged)-1].end {
				merged[len(merged)-1].end = removal.end
			}
			continue
		}
		merged = append(merged, removal)
	}
	for index := len(merged) - 1; index >= 0; index-- {
		removal := merged[index]
		content = content[:removal.start] + content[removal.end:]
	}
	return content
}

func formatNodeText(fset *token.FileSet, node ast.Node, content string) string {
	start := fset.PositionFor(node.Pos(), false).Offset
	end := fset.PositionFor(node.End(), false).Offset
	if start < 0 || end < start || end > len(content) {
		return ""
	}
	return content[start:end]
}

func formatGoCode(code string) (string, error) {
	// 创建 gofmt 命令
	cmd := exec.Command("gofmt")
	cmd.Stdin = strings.NewReader(code)

	// 获取输出
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("gofmt: %s; code=%q", strings.TrimSpace(string(exitErr.Stderr)), code)
		}
		return "", err
	}
	return string(output), nil
}

func writeFile(path string, content string) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// 创建目录
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	tmp, err := os.CreateTemp(dir, ".crud-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func writeGoFile(path, content string) error {
	formatted, err := formatGoCode(content)
	if err != nil {
		return err
	}
	return writeFile(path, canonicalizeGoContent(formatted))
}

func canonicalizeGoContent(content string) string {
	return strings.TrimRight(content, "\n") + "\n"
}

func writeWebLangFile(langEnData map[string]string, lang string, webLangDir WebDir) error {
	logicalTableName := strings.Join(append(slices.Clone(webLangDir.Path), webLangDir.LastName), "_")
	if IsProtectedTable(logicalTableName) {
		return fmt.Errorf("crud generation is forbidden for protected table %q", logicalTableName)
	}
	langTsContent := ""
	for k, v := range langEnData {
		quote := getQuote(v)
		keyStr := formatObjectKey(k)
		langTsContent += Tab(1) + keyStr + ": " + quote + v + quote + ",\n"
	}
	langTsContent = "export default {\n" + langTsContent + "}\n"
	path := filepath.Join(utils.RootPath(), webLangDir.LangFile(lang))
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
	apiRoute := indexVueData.RouteName
	if apiRoute == "" {
		apiRoute = utils.SnakeToCamel(handlerFile.LastName, false)
	}
	data["apiUrl"] = "'/admin/" + apiRoute + "/'"
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

func writeFormFile(formVueData FormVueData, webViewsDir WebDir, fields []crudmodel.Field, webTranslate string) error {
	formVueContent, err := renderFormFile(formVueData, fields, webTranslate)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(utils.RootPath(), webViewsDir.Views, "popupForm.vue"), formVueContent)
}

func renderFormFile(formVueData FormVueData, fields []crudmodel.Field, webTranslate string) (string, error) {
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
					message = ", message: " + strconv.Quote(field.Form.ValidatorMsg)
				}
				formValidatorRules[field.Name] = append(formValidatorRules[field.Name], "buildValidatorData({ name: "+strconv.Quote(item)+", title: t("+strconv.Quote(webTranslate+field.Name)+")"+message+" })")
			}
		}
	}
	data["formItemRules"] = buildFormValidatorRules(formValidatorRules)
	return assembleStub("html/form", data, false), nil
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
