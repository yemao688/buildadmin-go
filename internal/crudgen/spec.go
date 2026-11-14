package crudgen

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"buildadmin-go/internal/pkg/data_scope"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

type specFile struct {
	Name                 string         `mapstructure:"name"`
	Comment              string         `mapstructure:"comment"`
	Type                 string         `mapstructure:"type"`
	ModelFile            string         `mapstructure:"modelFile"`
	ControllerFile       string         `mapstructure:"controllerFile"`
	WebViewsDir          string         `mapstructure:"webViewsDir"`
	GenerateRelativePath string         `mapstructure:"generateRelativePath"`
	DatabaseConnection   string         `mapstructure:"databaseConnection"`
	IsCommonModel        int            `mapstructure:"isCommonModel"`
	Rebuild              string         `mapstructure:"rebuild"`
	QuickSearchField     []string       `mapstructure:"quickSearchField"`
	DefaultSortField     string         `mapstructure:"defaultSortField"`
	DefaultSortType      string         `mapstructure:"defaultSortType"`
	FormFields           *[]string      `mapstructure:"formFields"`
	ColumnFields         *[]string      `mapstructure:"columnFields"`
	RegisterOnly         bool           `mapstructure:"registerOnly"`
	DataScope            *specDataScope `mapstructure:"dataScope"`
	Indexes              []specIndex    `mapstructure:"indexes"`
	Fields               []specField    `mapstructure:"fields"`
	Menu                 *specMenu      `mapstructure:"menu"`
}

type specIndex struct {
	Name    string   `mapstructure:"name"`
	Unique  bool     `mapstructure:"unique"`
	Columns []string `mapstructure:"columns"`
}

type specDataScope struct {
	Mode            data_scope.Mode        `mapstructure:"mode"`
	OwnerColumn     string                 `mapstructure:"ownerColumn"`
	ReadExtraOwners []string               `mapstructure:"readExtraOwners"`
	AssignOnCreate  *bool                  `mapstructure:"assignOnCreate"`
	Reassignable    bool                   `mapstructure:"reassignable"`
	InheritFrom     *data_scope.InheritRef `mapstructure:"inheritFrom"`
}

type specMenu struct {
	Title  string `mapstructure:"title"`
	Parent int32  `mapstructure:"parent"`
	Weigh  *int32 `mapstructure:"weigh"`
}

type specField struct {
	Name              string              `mapstructure:"name"`
	Title             string              `mapstructure:"title"`
	Type              string              `mapstructure:"type"`
	DataType          string              `mapstructure:"dataType"`
	Length            int                 `mapstructure:"length"`
	Precision         int                 `mapstructure:"precision"`
	Default           string              `mapstructure:"default"`
	DefaultType       string              `mapstructure:"defaultType"`
	Null              bool                `mapstructure:"null"`
	PrimaryKey        bool                `mapstructure:"primaryKey"`
	Unsigned          bool                `mapstructure:"unsigned"`
	AutoIncrement     bool                `mapstructure:"autoIncrement"`
	Comment           string              `mapstructure:"comment"`
	DesignType        string              `mapstructure:"designType"`
	Form              crudmodel.FormAttr  `mapstructure:"form"`
	Table             crudmodel.TableAttr `mapstructure:"table"`
	FormBuildExclude  *bool               `mapstructure:"formBuildExclude"`
	TableBuildExclude bool                `mapstructure:"tableBuildExclude"`
}

// LoadSpec decodes an AI-authored YAML spec and applies safe generator
// defaults before the same security validation used by HTTP generation.
func LoadSpec(path string) (*GenerateOptions, error) {
	if path == "" {
		return nil, fmt.Errorf("spec path is required")
	}
	raw, err := decodeSpecFile(path)
	if err != nil {
		return nil, err
	}
	fields, err := buildSpecFields(raw.Fields)
	if err != nil {
		return nil, err
	}
	formFields := resolveSpecFormFields(raw.FormFields, fields)
	columnFields := resolveSpecColumnFields(raw.ColumnFields, fields)
	dataScope := resolveSpecDataScope(raw.DataScope)
	typeName := raw.Type
	if typeName == "" {
		typeName = "create"
	}
	table := crudmodel.Table{
		Name: raw.Name, Comment: raw.Comment, FormFields: formFields, ColumnFields: columnFields,
		QuickSearchField: raw.QuickSearchField, DefaultSortField: raw.DefaultSortField, DefaultSortType: raw.DefaultSortType,
		ModelFile: raw.ModelFile, ControllerFile: raw.ControllerFile, WebViewsDir: raw.WebViewsDir,
		GenerateRelativePath: raw.GenerateRelativePath, DatabaseConnection: raw.DatabaseConnection,
		IsCommonModel: raw.IsCommonModel, Rebuild: raw.Rebuild, DataScope: dataScope, RegisterOnly: raw.RegisterOnly,
	}
	indexes, err := parseSpecIndexes(raw.Indexes, fields)
	if err != nil {
		return nil, fmt.Errorf("spec %q indexes: %w", path, err)
	}
	table.Indexes = indexes
	if err := normalizeTableConfiguration(&table); err != nil {
		return nil, fmt.Errorf("spec %q validation failed: %w", path, err)
	}
	options := &GenerateOptions{Table: table, Fields: fields, Type: typeName}
	if raw.Menu != nil {
		options.Menu = &MenuOptions{Title: raw.Menu.Title, Parent: raw.Menu.Parent, Weigh: raw.Menu.Weigh}
	}
	if err := ValidateGenerationInput(table, fields); err != nil {
		return nil, fmt.Errorf("spec %q validation failed: %w", path, err)
	}
	return options, nil
}

// decodeSpecFile 读取 spec 文件并解码为 specFile：YAML 特殊键归一化、遗留
// cascadeOwners 检测与 Viper/mapstructure 解码的错误文本与顺序必须保持不变。
func decodeSpecFile(path string) (*specFile, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("read spec %q: %w", path, err)
	}
	content, err := os.ReadFile(path) //nolint:gosec // path 为开发者通过 CLI 显式传入的 crud_specs 文件路径，属本机生成流程
	if err != nil {
		return nil, fmt.Errorf("read spec %q: %w", path, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("parse spec %q: %w", path, err)
	}
	if err := detectLegacyCascadeOwners(document.Content[0]); err != nil {
		return nil, fmt.Errorf("spec %q: %w", path, err)
	}
	if err := normalizeNullKeys(&document); err != nil {
		return nil, fmt.Errorf("normalize spec %q: %w", path, err)
	}
	normalized, err := yaml.Marshal(&document)
	if err != nil {
		return nil, fmt.Errorf("normalize spec %q: %w", path, err)
	}
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader(normalized)); err != nil {
		return nil, fmt.Errorf("parse spec %q: %w", path, err)
	}
	var raw specFile
	if err := v.Unmarshal(&raw); err != nil {
		return nil, fmt.Errorf("decode spec %q: %w", path, err)
	}
	if raw.Name == "" {
		return nil, fmt.Errorf("spec name is required")
	}
	if len(raw.Fields) == 0 {
		return nil, fmt.Errorf("spec %q must define fields", raw.Name)
	}
	return &raw, nil
}

// buildSpecFields 按声明顺序把 spec 字段转换为实体字段，逐项保持默认值推导、
// designType 推断与校验的原始顺序。
func buildSpecFields(raw []specField) ([]crudmodel.Field, error) {
	fields := make([]crudmodel.Field, 0, len(raw))
	for i, item := range raw {
		if item.Name == "" {
			return nil, fmt.Errorf("field[%d] name is required", i)
		}
		field, err := buildSpecField(item)
		if err != nil {
			return nil, err
		}
		fields = append(fields, field)
	}
	return fields, nil
}

// buildSpecField 把单个 spec 字段转换为实体字段：默认值类型推导、时间字段表单
// 排除、designType 推断与字段校验。
func buildSpecField(item specField) (crudmodel.Field, error) {
	formBuildExclude := item.FormBuildExclude != nil && *item.FormBuildExclude
	field := crudmodel.Field{
		Name: item.Name, Title: item.Title, Type: strings.ToLower(item.Type), DataType: strings.ToLower(item.DataType),
		Length: item.Length, Precision: item.Precision, Default: item.Default, DefaultType: resolveSpecDefaultType(item), Null: item.Null,
		PrimaryKey: item.PrimaryKey, Unsigned: item.Unsigned, AutoIncrement: item.AutoIncrement,
		Comment: item.Comment, DesignType: item.DesignType, Form: item.Form, Table: item.Table,
		FormBuildExclude: formBuildExclude, TableBuildExclude: item.TableBuildExclude,
	}
	if item.FormBuildExclude == nil && isCanonicalTimeField(field.Name) {
		field.FormBuildExclude = true
	}
	if field.Type == "" {
		field.Type = field.DataType
	}
	if field.Type == "" {
		return crudmodel.Field{}, fmt.Errorf("field %q type is required", item.Name)
	}
	if field.DataType == "" && strings.Contains(field.Type, "(") {
		field.DataType = field.Type
	}
	if field.DesignType == "" {
		field.DesignType = inferDesignTypeForField(field)
	}
	applyDesignTypeDefaults(&field)
	normalizeFieldConfiguration(&field)
	if err := ValidateField(field); err != nil {
		return crudmodel.Field{}, fmt.Errorf("field %q: %w", item.Name, err)
	}
	return field, nil
}

// resolveSpecDefaultType 推导 defaultType：显式声明优先，其次是 default 的
// 关键字（null / empty string / none），其余非空值为 INPUT。
func resolveSpecDefaultType(item specField) string {
	defaultType := strings.ToUpper(strings.TrimSpace(item.DefaultType))
	if defaultType != "" {
		return defaultType
	}
	switch strings.ToLower(item.Default) {
	case "null":
		return "NULL"
	case "empty string":
		return "EMPTY STRING"
	case "none":
		return "NONE"
	}
	if item.Default == "" {
		return "NONE"
	}
	return "INPUT"
}

// resolveSpecFormFields 返回 spec 显式声明的表单字段；未声明时按字段定义推导
// （排除主键与表单排除字段）。
func resolveSpecFormFields(raw *[]string, fields []crudmodel.Field) []string {
	if raw != nil {
		return *raw
	}
	formFields := make([]string, 0, len(fields))
	for _, field := range fields {
		if !field.PrimaryKey && !field.FormBuildExclude {
			formFields = append(formFields, field.Name)
		}
	}
	return formFields
}

// resolveSpecColumnFields 返回 spec 显式声明的列表字段；未声明时使用全部字段。
func resolveSpecColumnFields(raw *[]string, fields []crudmodel.Field) []string {
	if raw != nil {
		return *raw
	}
	columnFields := make([]string, 0, len(fields))
	for _, field := range fields {
		columnFields = append(columnFields, field.Name)
	}
	return columnFields
}

// resolveSpecDataScope 返回 spec 的数据权限配置；未声明时使用 Auto 模式。
func resolveSpecDataScope(raw *specDataScope) *data_scope.Config {
	if raw == nil {
		return &data_scope.Config{Mode: data_scope.ModeAuto}
	}
	return &data_scope.Config{
		Mode:            raw.Mode,
		OwnerColumn:     raw.OwnerColumn,
		ReadExtraOwners: raw.ReadExtraOwners,
		AssignOnCreate:  raw.AssignOnCreate,
		Reassignable:    raw.Reassignable,
		InheritFrom:     raw.InheritFrom,
	}
}

// detectLegacyCascadeOwners 检测 spec 中遗留的 dataScope.cascadeOwners 声明：
// 方案 A 已移除主实体静态声明，子表继承改为 inheritFrom + 生成器自动维护主
// 实体 repo 的 CascadeOwners() 注册块，旧声明必须显式报错而不是静默忽略。
// key 匹配大小写不敏感（防 cascadeowners:/DataScope 等变体静默忽略）。
func detectLegacyCascadeOwners(root *yaml.Node) error {
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if !strings.EqualFold(root.Content[i].Value, "dataScope") {
			continue
		}
		value := root.Content[i+1]
		if value.Kind == yaml.AliasNode {
			value = value.Alias
		}
		if value == nil || value.Kind != yaml.MappingNode {
			return nil
		}
		for j := 0; j+1 < len(value.Content); j += 2 {
			if strings.EqualFold(value.Content[j].Value, "cascadeOwners") {
				return fmt.Errorf("cascadeOwners is no longer supported; declare inheritFrom on the child table spec and the parent registry is updated automatically")
			}
		}
	}
	return nil
}

// parseSpecIndexes 解析并校验 spec 的表级索引声明：索引名必填且为合法标识符、
// 全 spec 内不重复；至少一列且全部引用真实字段；单索引内列不重复。
func parseSpecIndexes(raw []specIndex, fields []crudmodel.Field) ([]crudmodel.IndexSpec, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	fieldNames := make(map[string]bool, len(fields))
	for _, field := range fields {
		fieldNames[field.Name] = true
	}
	indexes := make([]crudmodel.IndexSpec, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	// 字段类型速查：text/blob/json 家族建索引必须带前缀长度（MySQL 1170）。
	fieldType := make(map[string]string, len(fields))
	for _, field := range fields {
		fieldType[field.Name] = strings.ToLower(analyseFieldTypeForSpec(field))
	}
	for i, item := range raw {
		name, err := validateSpecIndexName(i, item.Name, seen)
		if err != nil {
			return nil, err
		}
		if len(item.Columns) == 0 {
			return nil, fmt.Errorf("index %q must declare at least one column", name)
		}
		columns, err := buildSpecIndexColumns(name, item, fieldNames, fieldType)
		if err != nil {
			return nil, err
		}
		indexes = append(indexes, crudmodel.IndexSpec{Name: name, Unique: item.Unique, Columns: columns})
	}
	return indexes, nil
}

// validateSpecIndexName 校验索引名：必填、合法标识符、非 MySQL 保留名，且全
// spec 内大小写不敏感唯一。返回去空格后的索引名并登记到 seen。
func validateSpecIndexName(index int, rawName string, seen map[string]bool) (string, error) {
	name := strings.TrimSpace(rawName)
	if name == "" {
		return "", fmt.Errorf("indexes[%d] name is required", index)
	}
	if err := ValidateIndexName(name); err != nil {
		return "", fmt.Errorf("indexes[%d] name %q: %w", index, name, err)
	}
	// PRIMARY 是 MySQL 主键索引保留名（错误 1280），必须在校验期拒绝。
	if strings.EqualFold(name, "PRIMARY") {
		return "", fmt.Errorf("indexes[%d] name %q is reserved; pick a different index name", index, name)
	}
	// MySQL 索引名大小写不敏感：uk_a 与 UK_A 视为重复。
	if seen[strings.ToLower(name)] {
		return "", fmt.Errorf("indexes[%d] duplicate index name %q", index, name)
	}
	seen[strings.ToLower(name)] = true
	return name, nil
}

// buildSpecIndexColumns 校验并展开单个索引的列声明：列必须引用真实字段、单
// 索引内不重复；前缀长度需为正且不超过 InnoDB utf8mb4 上限，无前缀的
// text/blob/json 家族列必须报错。
func buildSpecIndexColumns(name string, item specIndex, fieldNames map[string]bool, fieldType map[string]string) ([]string, error) {
	columns := make([]string, 0, len(item.Columns))
	columnSeen := make(map[string]bool, len(item.Columns))
	for _, col := range item.Columns {
		col = strings.TrimSpace(col)
		if col == "" {
			return nil, fmt.Errorf("index %q has an empty column", name)
		}
		baseName, prefix, err := parseIndexColumn(col)
		if err != nil {
			return nil, fmt.Errorf("index %q column %q: %w", name, col, err)
		}
		if !fieldNames[baseName] {
			return nil, fmt.Errorf("index %q column %q is not a spec field", name, baseName)
		}
		// 重复检测按基础列名判重：note(64) 与 note 是同一列。
		if columnSeen[baseName] {
			return nil, fmt.Errorf("index %q declares column %q more than once", name, baseName)
		}
		columnSeen[baseName] = true
		if prefix > 0 {
			// utf8mb4 下 InnoDB 单索引键上限 3072 字节（≈768 字符）；按字符数
			// 拒绝明显超界的前缀长度，具体字节上限仍由 MySQL 兜底。
			if prefix > 768 {
				return nil, fmt.Errorf("index %q column %q prefix length %d exceeds the 768-char InnoDB utf8mb4 limit", name, baseName, prefix)
			}
			columns = append(columns, fmt.Sprintf("%s(%d)", baseName, prefix))
		} else {
			// text/blob/json 家族无前缀 → MySQL 1170 拒绝；校验期必须失败。
			if isTextFamilyType(fieldType[baseName]) {
				return nil, fmt.Errorf("index %q column %q (%s) requires a prefix length, use col(N)", name, baseName, fieldType[baseName])
			}
			columns = append(columns, baseName)
		}
	}
	return columns, nil
}

// isTextFamilyType 报告列类型是否属于必须前缀长度的 text/blob/json 家族。
func isTextFamilyType(typ string) bool {
	switch typ {
	case "text", "tinytext", "mediumtext", "longtext",
		"blob", "tinyblob", "mediumblob", "longblob", "json":
		return true
	}
	return false
}

// parseIndexColumn 解析索引列声明，支持 MySQL 前缀索引语法 `col(N)`：
// 返回基础列名与前缀长度（无前缀时为 0）。N 必须是正整数。
func parseIndexColumn(raw string) (string, int, error) {
	open := strings.IndexByte(raw, '(')
	if open < 0 {
		return raw, 0, nil
	}
	if !strings.HasSuffix(raw, ")") {
		return "", 0, fmt.Errorf("malformed prefix syntax %q, want col(N)", raw)
	}
	base := strings.TrimSpace(raw[:open])
	prefixStr := strings.TrimSpace(raw[open+1 : len(raw)-1])
	if base == "" {
		return "", 0, fmt.Errorf("missing column name in %q", raw)
	}
	prefix, err := strconv.Atoi(prefixStr)
	if err != nil || prefix <= 0 {
		return "", 0, fmt.Errorf("prefix length in %q must be a positive integer", raw)
	}
	return base, prefix, nil
}

// normalizeNullKeys fixes YAML's special null key token before Viper sees it.
// Without this pass, an unquoted `null:` key can disappear during mapstructure
// decoding even though its value is a valid boolean.
func normalizeNullKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if err := normalizeNullKeyPair(node.Content[i], node.Content[i+1]); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range node.Content {
		if err := normalizeNullKeys(child); err != nil {
			return err
		}
	}
	return nil
}

// normalizeNullKeyPair 归一化单个 mapping 键值对并递归其值，处理顺序不可调整。
func normalizeNullKeyPair(key, value *yaml.Node) error {
	normalizeNullMappingKey(key)
	normalizeNullDefaultTypeValue(key, value)
	normalizeBooleanMultiFlag(key, value)
	if err := normalizeComSearchInputAttr(key, value); err != nil {
		return err
	}
	return normalizeNullKeys(value)
}

// normalizeNullMappingKey 把 YAML 的 null/~ 键还原为字符串键 "null"。
func normalizeNullMappingKey(key *yaml.Node) {
	if key.Kind == yaml.ScalarNode && key.Tag == "!!null" && (key.Value == "null" || key.Value == "~") {
		key.Tag = "!!str"
		key.Value = "null"
	}
}

// normalizeNullDefaultTypeValue 把 defaultType 的裸 null 值还原为 "NULL" 字面量。
func normalizeNullDefaultTypeValue(key, value *yaml.Node) {
	if key.Value == "defaultType" && value.Kind == yaml.ScalarNode && value.Tag == "!!null" {
		value.Tag = "!!str"
		value.Value = "NULL"
	}
}

// normalizeBooleanMultiFlag 把多选布尔标记的 true/false 归一为 "1"/""，
// 避免 mapstructure 把 bool 与空串混同。
func normalizeBooleanMultiFlag(key, value *yaml.Node) {
	if value.Kind != yaml.ScalarNode || value.Tag != "!!bool" {
		return
	}
	switch key.Value {
	case "selectMulti", "imageMulti", "fileMulti":
		value.Tag = "!!str"
		if value.Value == "true" {
			value.Value = "1"
		} else {
			value.Value = ""
		}
	}
}

// normalizeComSearchInputAttr 把 comSearchInputAttr 的字符串值编码为规范化
// YAML 节点，保持与历史归一化结果一致的输出。
func normalizeComSearchInputAttr(key, value *yaml.Node) error {
	if key.Value != "comSearchInputAttr" || value.Kind != yaml.ScalarNode {
		return nil
	}
	attrs, err := crudmodel.ParseComSearchInputAttrs(value.Value)
	if err != nil {
		return err
	}
	encoded, _ := yaml.Marshal(attrs)
	var replacement yaml.Node
	if yaml.Unmarshal(encoded, &replacement) == nil && len(replacement.Content) == 1 {
		*value = *replacement.Content[0]
	}
	return nil
}

// inferDesignTypeForField follows Helper.php::$inputTypeRule in order.  The
// order is significant: suffix rules intentionally beat broad type rules.
func inferDesignTypeForField(field crudmodel.Field) string {
	name := strings.ToLower(field.Name)
	typ := strings.ToLower(analyseFieldTypeForSpec(field))
	columnType := specFieldColumnType(field)
	// Normalize type+length to the full column type so length:1 is treated
	// identically to tinyint(1)/char(1): status/state/type then hit the radio
	// rule below, switch/toggle suffixes hit switch, and neither depends on
	// the author writing the parenthesized dataType.
	if columnType != "" && !strings.Contains(columnType, "(") && field.Length == 1 &&
		slicesContains([]string{"tinyint", "char"}, columnType) {
		columnType = columnType + "(1)"
	}

	if designType, ok := inferSpecIdentityDesignType(field, name); ok {
		return designType
	}
	if designType, ok := inferSpecWidgetDesignType(typ, columnType, name); ok {
		return designType
	}
	if designType, ok := inferSpecTemporalDesignType(typ); ok {
		return designType
	}
	if designType, ok := inferSpecSuffixDesignType(name); ok {
		return designType
	}
	if designType, ok := inferSpecColumnDesignType(columnType, typ, name); ok {
		return designType
	}
	return "string"
}

// specFieldColumnType 返回用于 designType 推断的完整列类型（dataType 优先，
// 缺省回退 type），统一小写。
func specFieldColumnType(field crudmodel.Field) string {
	columnType := strings.ToLower(field.DataType)
	if columnType == "" {
		columnType = strings.ToLower(field.Type)
	}
	return columnType
}

// inferSpecIdentityDesignType 处理主键、weigh 与规范时间字段，优先级最高。
func inferSpecIdentityDesignType(field crudmodel.Field, name string) (string, bool) {
	if field.AutoIncrement && strings.Contains(name, "id") {
		return "pk", true
	}
	if name == "weigh" {
		return "weigh", true
	}
	if isCanonicalTimeField(name) {
		return "timestamp", true
	}
	return "", false
}

// inferSpecWidgetDesignType 处理依赖 db 类型与字段后缀组合的控件类型。
func inferSpecWidgetDesignType(typ, columnType, name string) (string, bool) {
	if matchesTypeAndSuffix(typ, name, []string{"tinyint", "int", "enum"}, []string{"switch", "toggle"}) ||
		matchesColumnTypeAndSuffix(columnType, name, []string{"tinyint(1)", "char(1)", "tinyint(1) unsigned"}, []string{"switch", "toggle"}) {
		return "switch", true
	}
	if matchesTypeAndSuffix(typ, name, []string{"longtext", "text", "mediumtext", "smalltext", "tinytext", "bigtext"}, []string{"content", "editor"}) {
		return "editor", true
	}
	if matchesTypeAndSuffix(typ, name, []string{"varchar"}, []string{"textarea", "multiline", "rows"}) {
		return "textarea", true
	}
	if hasSuffix(name, []string{"array"}) {
		return "array", true
	}
	if matchesTypeAndSuffix(typ, name, []string{"int"}, []string{"time", "datetime"}) {
		return "timestamp", true
	}
	return "", false
}

// inferSpecTemporalDesignType 把日期时间类 db 类型映射为对应控件。
func inferSpecTemporalDesignType(typ string) (string, bool) {
	switch typ {
	case "datetime", "timestamp":
		return "datetime", true
	case "date":
		return "date", true
	case "year":
		return "year", true
	case "time":
		return "time", true
	}
	return "", false
}

// inferSpecSuffixDesignType 按字段名后缀推断控件，顺序对齐 Helper.php。
func inferSpecSuffixDesignType(name string) (string, bool) {
	if hasSuffix(name, []string{"select", "list", "data"}) {
		return "select", true
	}
	if hasSuffix(name, []string{"selects", "multi", "lists"}) {
		return "selects", true
	}
	if hasSuffix(name, []string{"_ids"}) {
		return "remoteSelects", true
	}
	if hasSuffix(name, []string{"_id"}) {
		return "remoteSelect", true
	}
	if hasSuffix(name, []string{"city"}) {
		return "city", true
	}
	if hasSuffix(name, []string{"image", "avatar"}) {
		return "image", true
	}
	if hasSuffix(name, []string{"images", "avatars"}) {
		return "images", true
	}
	if hasSuffix(name, []string{"file"}) {
		return "file", true
	}
	if hasSuffix(name, []string{"files"}) {
		return "files", true
	}
	if hasSuffix(name, []string{"icon"}) {
		return "icon", true
	}
	return "", false
}

// inferSpecColumnDesignType 按列类型兜底推断控件（radio/number/textarea 等）。
func inferSpecColumnDesignType(columnType, typ, name string) (string, bool) {
	if matchesColumnTypeAndSuffix(columnType, name, []string{"tinyint(1)", "char(1)", "tinyint(1) unsigned"}, []string{"status", "state", "type"}) {
		return "radio", true
	}
	if hasSuffix(name, []string{"number", "int", "num"}) {
		return "number", true
	}
	if slicesContains([]string{"bigint", "int", "mediumint", "smallint", "tinyint", "decimal", "double", "float"}, typ) {
		return "number", true
	}
	if slicesContains([]string{"longtext", "text", "mediumtext", "smalltext", "tinytext", "bigtext"}, typ) {
		return "textarea", true
	}
	if typ == "enum" {
		return "radio", true
	}
	if typ == "set" {
		return "checkbox", true
	}
	if hasSuffix(name, []string{"color"}) {
		return "color", true
	}
	return "", false
}

func analyseFieldTypeForSpec(field crudmodel.Field) string {
	typ := field.Type
	if field.DataType != "" {
		typ = field.DataType
	}
	if i := strings.IndexByte(typ, '('); i >= 0 {
		typ = typ[:i]
	}
	return strings.TrimSpace(typ)
}

func hasSuffix(name string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func matchesTypeAndSuffix(typ, name string, types, suffixes []string) bool {
	return slicesContains(types, typ) && hasSuffix(name, suffixes)
}

func matchesColumnTypeAndSuffix(columnType, name string, types, suffixes []string) bool {
	return slicesContains(types, columnType) && hasSuffix(name, suffixes)
}

func slicesContains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func isCanonicalTimeField(name string) bool {
	switch strings.ToLower(name) {
	case "createtime", "updatetime", "create_time", "update_time":
		return true
	default:
		return false
	}
}

// specTableDefault 是某个 designType 对应的 table 列默认配置；字段已有非零值
// 时不覆盖。
type specTableDefault struct {
	Render     string
	Operator   string
	Sortable   string
	Search     string
	Width      int
	TimeFormat string
}

var specTableDefaultsByDesignType = map[string]specTableDefault{
	"pk":            {"", "RANGE", "custom", "", 70, ""},
	"spk":           {"", "RANGE", "custom", "", 180, ""},
	"weigh":         {"", "RANGE", "custom", "", 0, ""},
	"timestamp":     {"datetime", "RANGE", "custom", "datetime", 160, "yyyy-mm-dd hh:MM:ss"},
	"string":        {"none", "LIKE", "false", "", 0, ""},
	"password":      {"", "false", "", "", 0, ""},
	"number":        {"none", "RANGE", "false", "", 0, ""},
	"float":         {"none", "RANGE", "false", "", 0, ""},
	"radio":         {"tag", "eq", "false", "", 0, ""},
	"checkbox":      {"tags", "FIND_IN_SET", "false", "", 0, ""},
	"switch":        {"switch", "eq", "false", "", 0, ""},
	"textarea":      {"", "false", "", "", 0, ""},
	"array":         {"", "false", "", "", 0, ""},
	"datetime":      {"", "RANGE", "custom", "datetime", 160, ""},
	"year":          {"", "RANGE", "custom", "", 0, ""},
	"date":          {"", "RANGE", "custom", "date", 0, ""},
	"time":          {"", "RANGE", "custom", "time", 0, ""},
	"select":        {"tag", "eq", "false", "", 0, ""},
	"selects":       {"tags", "FIND_IN_SET", "false", "", 0, ""},
	"remoteSelect":  {"tags", "LIKE", "", "string", 0, ""},
	"remoteSelects": {"tags", "FIND_IN_SET", "", "remoteSelect", 0, ""},
	"editor":        {"", "false", "", "", 0, ""},
	"city":          {"", "false", "", "", 0, ""},
	"image":         {"image", "false", "", "", 0, ""},
	"images":        {"images", "false", "", "", 0, ""},
	"file":          {"none", "false", "", "", 0, ""},
	"files":         {"none", "false", "", "", 0, ""},
	"icon":          {"icon", "false", "", "", 0, ""},
	"color":         {"color", "false", "", "", 0, ""},
}

var specValidatorDefaultsByDesignType = map[string]string{
	"timestamp": "date",
	"password":  "password",
	"number":    "number",
	"float":     "float",
	"datetime":  "date",
	"year":      "date",
	"date":      "date",
	"editor":    "editorRequired",
}

var specStepDesignTypes = map[string]bool{"number": true, "float": true}

func applyDesignTypeDefaults(field *crudmodel.Field) {
	if defaults, ok := specTableDefaultsByDesignType[field.DesignType]; ok {
		applySpecTableDefaults(field, defaults)
	}
	applySpecValidatorDefaults(field)
	applySpecStepDefaults(field)
	applySpecMultiFlagDefaults(field)
	applySpecRemoteDefaults(field)
	applySpecTextareaDefaults(field)
}

// applySpecTableDefaults 仅在 table 列属性为空/零值时写入 designType 默认值。
func applySpecTableDefaults(field *crudmodel.Field, defaults specTableDefault) {
	if field.Table.Render == "" {
		field.Table.Render = defaults.Render
	}
	if field.Table.Operator == "" {
		field.Table.Operator = defaults.Operator
	}
	if field.Table.Sortable == "" {
		field.Table.Sortable = defaults.Sortable
	}
	if field.Table.ComSearchRender == "" {
		field.Table.ComSearchRender = defaults.Search
	}
	if field.Table.Width == 0 {
		field.Table.Width = defaults.Width
	}
	if field.Table.TimeFormat == "" {
		field.Table.TimeFormat = defaults.TimeFormat
	}
}

// applySpecValidatorDefaults 为未声明 validator 的 designType 写入默认校验器。
func applySpecValidatorDefaults(field *crudmodel.Field) {
	validator, ok := specValidatorDefaultsByDesignType[field.DesignType]
	if !ok || len(field.Form.Validator) != 0 {
		return
	}
	field.Form.Validator = []string{validator}
}

// applySpecStepDefaults 为 number/float 字段补默认步长 1。
func applySpecStepDefaults(field *crudmodel.Field) {
	if !specStepDesignTypes[field.DesignType] || field.Form.Step != 0 {
		return
	}
	field.Form.Step = 1
}

// applySpecMultiFlagDefaults 为多选类 designType 打开对应的多选标记。
func applySpecMultiFlagDefaults(field *crudmodel.Field) {
	switch field.DesignType {
	case "selects", "remoteSelects":
		if field.Form.SelectMulti == "" {
			field.Form.SelectMulti = "1"
		}
	case "images":
		if field.Form.ImageMulti == "" {
			field.Form.ImageMulti = "1"
		}
	case "files":
		if field.Form.FileMulti == "" {
			field.Form.FileMulti = "1"
		}
	}
}

// applySpecRemoteDefaults 为远程下拉字段补 pk/展示字段默认值。
func applySpecRemoteDefaults(field *crudmodel.Field) {
	if field.DesignType != "remoteSelect" && field.DesignType != "remoteSelects" {
		return
	}
	if field.Form.RemotePk == "" {
		field.Form.RemotePk = "id"
	}
	if field.Form.RemoteField == "" {
		field.Form.RemoteField = "name"
	}
}

// applySpecTextareaDefaults 为 textarea 字段补默认行数 3。
func applySpecTextareaDefaults(field *crudmodel.Field) {
	if field.DesignType != "textarea" || field.Form.Rows != 0 {
		return
	}
	field.Form.Rows = 3
}
