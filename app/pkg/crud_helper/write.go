package crud_helper

import (
	"bytes"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/utils"
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

func writeModelFile(db *gorm.DB, tablePk string, fullTableName string, tableName string, modelData ModelData, modelFile NameInfo) (string, error) {
	if tablePk != "" {
		modelData.Pk = tablePk
	}
	structContent, err := getGenerateStruct(db, fullTableName, tableName, modelData.ModelFieldType)
	if err != nil {
		return "", err
	}
	modelData.StructTemp = addCityTextFields(structContent, modelData.CityTextFields)
	modelData.StructTemp = addRelationFields(modelData.StructTemp, modelData.RelationFields)
	prepareModelTimestampData(&modelData)

	modelContent, err := render(modelFile.ParseFile, modelTemp, modelData)
	if err != nil {
		return "", err
	}
	if err := writeGoFile(modelFile.ParseFile, modelContent); err != nil {
		return "", err
	}

	if err := writeProvider(modelFile.RootFileName, modelData.ClassName+"Model"); err != nil {
		return "", err
	}
	return structContent, nil
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

func getGenerateStruct(db *gorm.DB, fullTableName string, tableName string, fieldTypeOverrides map[string]string) (string, error) {
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
	data := g.GenerateModelAs(fullTableName, utils.SnakeToCamel(tableName, true), options...)

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

func writeHandlerFile(handlerData HandlerData, handlerFile NameInfo, structContent string) error {
	//请求参数验证结构体
	index := strings.Index(structContent, "struct {")
	if index == -1 {
		return nil
	}
	validateContent := "type " + handlerData.ClassName + "Param " + structContent[index:]

	re := regexp.MustCompile(`gorm:"[^"]*" `)
	validateContent = re.ReplaceAllString(validateContent, "")
	validateContent = rewriteFlexNumericParamFields(validateContent, handlerData.ParamTypeOverrides)
	handlerData.ValidateParam = excludeParamFieldsWithPrimaryKey(validateContent, handlerData.ExcludeParamFields, handlerData.PkJSONName)

	//渲染文件内容
	handlerContent, err := render(handlerFile.ParseFile, handlerTemp, handlerData)
	if err != nil {
		return err
	}
	//写入文件
	if err := writeGoFile(handlerFile.ParseFile, handlerContent); err != nil {
		return err
	}
	//写入provider
	if err := writeProvider(handlerFile.RootFileName, handlerData.ClassName+"Handler"); err != nil {
		return err
	}
	if err := writeRegistrarFile(handlerData, handlerFile); err != nil {
		return err
	}
	if err := writeProvider(handlerFile.RootFileName, handlerData.ClassName+"Registrar"); err != nil {
		return err
	}
	if err := writeRegistrarProviderEntry(handlerData.ClassName); err != nil {
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

// renderModel renders the model template to a string for tests.
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

// renderHandler renders the handler template to a string for tests. It mirrors
// the validate-param derivation in writeHandlerFile without touching the disk.
func renderHandler(handlerData HandlerData, structContent string) (string, error) {
	if handlerData.PkJSONName == "" {
		handlerData.PkJSONName = "id"
	}
	index := strings.Index(structContent, "struct {")
	if index != -1 {
		validateContent := "type " + handlerData.ClassName + "Param " + structContent[index:]
		re := regexp.MustCompile(`gorm:"[^"]*" `)
		validateContent = re.ReplaceAllString(validateContent, "")
		validateContent = rewriteFlexNumericParamFields(validateContent, handlerData.ParamTypeOverrides)
		handlerData.ValidateParam = excludeParamFieldsWithPrimaryKey(validateContent, handlerData.ExcludeParamFields, handlerData.PkJSONName)
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

func writeRegistrarFile(handlerData HandlerData, handlerFile NameInfo) error {
	registrarPath := registrarFilePath(handlerFile)
	data := RegistrarData{
		Namespace: handlerFile.Namespace,
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

func registrarFilePath(handlerFile NameInfo) string {
	return strings.TrimSuffix(handlerFile.ParseFile, filepath.Ext(handlerFile.ParseFile)) + "_route.go"
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	return strings.ToLower(value[:1]) + value[1:]
}

func writeRegistrarProviderEntry(name string) error {
	path := filepath.Join(utils.RootPath(), "router", "registrar_set.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, err := addRegistrarProviderEntry(string(content), name)
	if err != nil {
		return err
	}
	return writeGoFile(path, updated)
}

func addRegistrarProviderEntry(content, name string) (string, error) {
	registrarType := name + "Registrar"
	registrarVar := lowerFirst(name) + "Registrar"
	param := "\t" + registrarVar + " *admin." + registrarType + ",\n"
	entry := "\t\t" + registrarVar + ",\n"

	if !strings.Contains(content, param) {
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

func RemoveRegistrarProvider(name string) error {
	path := filepath.Join(utils.RootPath(), "router", "registrar_set.go")
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updated, err := removeRegistrarProviderEntry(string(content), name)
	if err != nil {
		return err
	}
	return writeGoFile(path, updated)
}

func removeRegistrarProviderEntry(content, name string) (string, error) {
	registrarType := name + "Registrar"
	registrarVar := lowerFirst(name) + "Registrar"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", content, parser.ParseComments)
	if err != nil {
		return "", err
	}
	var removals []sourceRemoval
	ast.Inspect(file, func(node ast.Node) bool {
		function, ok := node.(*ast.FuncDecl)
		if !ok || function.Name == nil || function.Name.Name != "ProvideRegistrars" || function.Type.Params == nil {
			return true
		}
		params := make([]ast.Node, 0, len(function.Type.Params.List))
		for _, field := range function.Type.Params.List {
			if len(field.Names) == 1 && field.Names[0].Name == registrarVar && strings.Contains(formatNodeText(fset, field, content), registrarType) {
				continue
			}
			params = append(params, field)
		}
		if len(params) != len(function.Type.Params.List) {
			removals = append(removals, removeListElementRanges(content, fset, function.Type.Params.Opening, function.Type.Params.Closing, fieldsAsNodes(function.Type.Params.List), func(node ast.Node) bool {
				field, ok := node.(*ast.Field)
				return ok && len(field.Names) == 1 && field.Names[0].Name == registrarVar && strings.Contains(formatNodeText(fset, field, content), registrarType)
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
			return ok && ident.Name == registrarVar
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

func writeProvider(dir string, name string) error {
	providerPath := filepath.Join(utils.RootPath(), dir, "provider.go")
	if err := ValidateGeneratedAbsolutePath(providerPath, "app/admin/model", "app/common/model", "app/admin/handler"); err != nil {
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
	content = []byte(string(content)[:lastIndex] + "	New" + name + ",\n)")
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

func writeFormFile(formVueData FormVueData, webViewsDir WebDir, fields []model.Field, webTranslate string) error {
	formVueContent, err := renderFormFile(formVueData, fields, webTranslate)
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(utils.RootPath(), webViewsDir.Views, "popupForm.vue"), formVueContent)
}

func renderFormFile(formVueData FormVueData, fields []model.Field, webTranslate string) (string, error) {
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
