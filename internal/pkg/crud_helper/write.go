package crud_helper

import (
	crudmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/util"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
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
func writeModelFiles(db *gorm.DB, tablePk string, fullTableName string, tableName string, modelData ModelData, entityFile, repositoryFile NameInfo, fields []crudmodel.Field, skipRepo bool) (string, error) {
	if tablePk != "" {
		modelData.Pk = tablePk
	}
	structContent, err := getGenerateStruct(db, fullTableName, modelData.ClassName, modelData.ModelFieldType)
	if err != nil {
		return "", err
	}
	structContent = applySpecDefaultTags(structContent, fields)
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

	if !skipRepo {
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
		goName := util.SnakeToCamel(field+"_text", true)
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
		FieldWithIndexTag: true,
		//if you want to generate type tags from database, set FieldWithTypeTag true
		FieldWithTypeTag: true,
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

// applySpecDefaultTags 把 spec 字段声明的 default（EMPTY STRING/INPUT）映射进
// 生成 struct 的 gorm tag。gorm/gen 从数据库 introspection 生成时，对 DEFAULT ''
// （string 零值）和 DEFAULT 0（int 零值）不产出 default tag（needDefaultTag
// 语义），导致生成 model 与 spec 不一致、全新库 apply 被 default 差异阻塞。
// 此函数按 spec 补齐；gen 已产出的（非零值）保持原样不重复插入。
func applySpecDefaultTags(structContent string, fields []crudmodel.Field) string {
	for _, field := range fields {
		if field.PrimaryKey {
			continue
		}
		defaultTag := specDefaultGormTag(field)
		if defaultTag == "" {
			continue
		}
		structContent = ensureFieldDefaultTag(structContent, field.Name, defaultTag)
	}
	return structContent
}

// specDefaultGormTag 把 spec 字段的 default 声明映射为 gorm tag 片段。
// 数值类型不加引号（default:0），字符串类型加单引号（default:''）。
func specDefaultGormTag(field crudmodel.Field) string {
	switch field.DefaultType {
	case "EMPTY STRING":
		return "default:''"
	case "INPUT":
		if field.Default == "" {
			return ""
		}
		if isNumericSpecField(field) {
			return "default:" + field.Default
		}
		return "default:'" + field.Default + "'"
	default:
		return ""
	}
}

// isNumericSpecField 报告 spec 字段是否为数值类型（int/decimal/float 家族）。
func isNumericSpecField(field crudmodel.Field) bool {
	typ := strings.ToLower(analyseFieldTypeForSpec(field))
	for _, prefix := range []string{"int", "decimal", "float", "double", "tinyint", "smallint", "mediumint", "bigint", "bool"} {
		if strings.HasPrefix(typ, prefix) {
			return true
		}
	}
	return false
}

// ensureFieldDefaultTag 在 gorm tag 的 column 段后插入 default 片段；
// tag 已含 default: 时跳过（避免与 gen 产出的非零值重复）。
func ensureFieldDefaultTag(structContent, columnName, defaultTag string) string {
	re := regexp.MustCompile(`(gorm:"column:` + regexp.QuoteMeta(columnName) + `;)([^"]*)(")`)
	return re.ReplaceAllStringFunc(structContent, func(m string) string {
		if strings.Contains(m, "default:") {
			return m
		}
		parts := re.FindStringSubmatch(m)
		return parts[1] + defaultTag + ";" + parts[2] + parts[3]
	})
}

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

func writeHandlerFile(handlerData HandlerData, handlerFile NameInfo, structContent string, dtoFile, registrarFile NameInfo) error {
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
		name := AtomicRouteCapabilityName(handlerData.RouteName)
		if name == "" {
			name = AtomicRouteCapabilityName(lowerFirst(handlerData.ClassName))
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
				"bool":    "validator.FlexBool",
				"int32":   "validator.FlexInt32",
				"int64":   "validator.FlexInt64",
				"float64": "validator.FlexFloat64",
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
		Namespace: registrarFile.Namespace,
		ClassName: handlerData.ClassName,
		RouteName: lowerFirst(handlerData.ClassName),
		RoutePath: handlerData.RouteName,
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

// adminRouterProviderPath 是 admin 渠道路由注册器的中心锚点文件：
// ProviderSet（NewXxxRegistrar 列表）与 ProvideRegistrars（每模块一行 + handler 参数）。
func adminRouterProviderPath() string {
	return filepath.Join(util.RootPath(), "internal", "admin", "router", "provider.go")
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

// addProvideRegistrarsEntry 以 AST 精确定位 ProvideRegistrars FuncDecl：
// 参数插到其参数列表右括号前，返回条目插到其 return 的 []Registrar 字面量
// 右花括号前。文件里其它函数（即使签名相似）不受影响。
func addProvideRegistrarsEntry(content, className string) (string, error) {
	if strings.Contains(content, "New"+className+"Registrar(") {
		return content, nil
	}
	varName := lowerFirst(className)
	param := "\t" + varName + " *handler." + className + "Handler,\n"
	entry := "\t\tNew" + className + "Registrar(" + varName + "),\n"

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", err
	}
	paramInsert, entryInsert := -1, -1
	ast.Inspect(file, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Name == nil || function.Name.Name != "ProvideRegistrars" || function.Body == nil {
			return true
		}
		if function.Type.Params != nil {
			paramInsert = fset.PositionFor(function.Type.Params.Closing, false).Offset
		}
		ast.Inspect(function.Body, func(n ast.Node) bool {
			stmt, ok := n.(*ast.ReturnStmt)
			if !ok || len(stmt.Results) != 1 {
				return true
			}
			lit, ok := stmt.Results[0].(*ast.CompositeLit)
			if !ok {
				return true
			}
			entryInsert = fset.PositionFor(lit.Rbrace, false).Offset
			return false
		})
		return false
	})
	if paramInsert < 0 {
		return "", fmt.Errorf("ProvideRegistrars signature anchor not found")
	}
	if entryInsert < 0 {
		return "", fmt.Errorf("ProvideRegistrars return anchor not found")
	}
	content = content[:paramInsert] + param + content[paramInsert:]
	entryInsert += len(param)
	content = content[:entryInsert] + entry + content[entryInsert:]
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
	// 2. 只扫 ProvideRegistrars 函数体 return 的 []Registrar 字面量
	ast.Inspect(file, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Name == nil || function.Name.Name != "ProvideRegistrars" || function.Body == nil {
			return true
		}
		ast.Inspect(function.Body, func(n ast.Node) bool {
			stmt, ok := n.(*ast.ReturnStmt)
			if !ok || len(stmt.Results) != 1 {
				return true
			}
			composite, ok := stmt.Results[0].(*ast.CompositeLit)
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
			return false
		})
		return false
	})
	// 3. 真正的 wire.NewSet ProviderSet 中的 NewXxxRegistrar 条目
	// （callee 必须是 wire.NewSet，防止误伤同名调用）。
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != "NewSet" {
			return true
		}
		xIdent, ok := selector.X.(*ast.Ident)
		if !ok || xIdent.Name != "wire" {
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
		// F5：callee 必须是 wire.NewSet，防止把同名调用误当 ProviderSet。
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || selector.Sel.Name != "NewSet" {
			return true
		}
		xIdent, ok := selector.X.(*ast.Ident)
		if !ok || xIdent.Name != "wire" {
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
	providerPath := filepath.Join(util.RootPath(), dir, "provider.go")
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
	content, err := os.ReadFile(filepath.Join(util.RootPath(), dir, "provider.go"))
	if err != nil {
		return err
	}
	newContent, err := removeProviderEntry(string(content), name)
	if err != nil {
		return err
	}
	return writeGoFile(filepath.Join(util.RootPath(), dir, "provider.go"), newContent)
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
	path := filepath.Join(util.RootPath(), webLangDir.LangFile(lang))
	return writeFile(path, buildLangTsContent(langEnData))
}

// buildLangTsContent 组装语言 .ts 文件内容。keys 必须排序后迭代：Go map 的
// 迭代顺序由运行时随机决定，直接 range 会导致每次生成的语言文件键序不同，
// 破坏"crud:delete + 重新生成逐字节一致"的契约（重新生成 diff 噪音无法审查）。
func buildLangTsContent(langEnData map[string]string) string {
	keys := make([]string, 0, len(langEnData))
	for k := range langEnData {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	langTsContent := ""
	for _, k := range keys {
		v := langEnData[k]
		quote := getQuote(v)
		keyStr := formatObjectKey(k)
		langTsContent += Tab(1) + keyStr + ": " + quote + v + quote + ",\n"
	}
	return "export default {\n" + langTsContent + "}\n"
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
		apiRoute = util.SnakeToCamel(handlerFile.LastName, false)
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
	return writeFile(filepath.Join(util.RootPath(), webViewsDir.Views, "index.vue"), indexVueContent)
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
	return writeFile(filepath.Join(util.RootPath(), webViewsDir.Views, "popupForm.vue"), formVueContent)
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
