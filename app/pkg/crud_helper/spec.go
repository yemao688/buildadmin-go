package crud_helper

import (
	"bytes"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	"os"
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
	DataScope            *specDataScope `mapstructure:"dataScope"`
	Fields               []specField    `mapstructure:"fields"`
	Menu                 *specMenu      `mapstructure:"menu"`
}

type specDataScope struct {
	Mode           data_scope.Mode `mapstructure:"mode"`
	OwnerColumn    string          `mapstructure:"ownerColumn"`
	AssignOnCreate *bool           `mapstructure:"assignOnCreate"`
}

type specMenu struct {
	Title  string `mapstructure:"title"`
	Parent int32  `mapstructure:"parent"`
	Weigh  *int32 `mapstructure:"weigh"`
}

type specField struct {
	Name              string          `mapstructure:"name"`
	Title             string          `mapstructure:"title"`
	Type              string          `mapstructure:"type"`
	DataType          string          `mapstructure:"dataType"`
	Length            int             `mapstructure:"length"`
	Precision         int             `mapstructure:"precision"`
	Default           string          `mapstructure:"default"`
	DefaultType       string          `mapstructure:"defaultType"`
	Null              bool            `mapstructure:"null"`
	PrimaryKey        bool            `mapstructure:"primaryKey"`
	Unsigned          bool            `mapstructure:"unsigned"`
	AutoIncrement     bool            `mapstructure:"autoIncrement"`
	Comment           string          `mapstructure:"comment"`
	DesignType        string          `mapstructure:"designType"`
	Form              model.FormAttr  `mapstructure:"form"`
	Table             model.TableAttr `mapstructure:"table"`
	FormBuildExclude  *bool           `mapstructure:"formBuildExclude"`
	TableBuildExclude bool            `mapstructure:"tableBuildExclude"`
}

// LoadSpec decodes an AI-authored YAML spec and applies safe generator
// defaults before the same security validation used by HTTP generation.
func LoadSpec(path string) (*GenerateOptions, error) {
	if path == "" {
		return nil, fmt.Errorf("spec path is required")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("read spec %q: %w", path, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec %q: %w", path, err)
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("parse spec %q: %w", path, err)
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

	fields := make([]model.Field, 0, len(raw.Fields))
	for i, item := range raw.Fields {
		if item.Name == "" {
			return nil, fmt.Errorf("field[%d] name is required", i)
		}
		formBuildExclude := item.FormBuildExclude != nil && *item.FormBuildExclude
		defaultType := strings.ToUpper(strings.TrimSpace(item.DefaultType))
		if defaultType == "" {
			switch strings.ToLower(item.Default) {
			case "null":
				defaultType = "NULL"
			case "empty string":
				defaultType = "EMPTY STRING"
			case "none":
				defaultType = "NONE"
			default:
				if item.Default == "" {
					defaultType = "NONE"
				} else {
					defaultType = "INPUT"
				}
			}
		}
		field := model.Field{
			Name: item.Name, Title: item.Title, Type: strings.ToLower(item.Type), DataType: strings.ToLower(item.DataType),
			Length: item.Length, Precision: item.Precision, Default: item.Default, DefaultType: defaultType, Null: item.Null,
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
			return nil, fmt.Errorf("field %q type is required", item.Name)
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
			return nil, fmt.Errorf("field %q: %w", item.Name, err)
		}
		fields = append(fields, field)
	}

	formFields := []string(nil)
	if raw.FormFields != nil {
		formFields = *raw.FormFields
	}
	if raw.FormFields == nil {
		formFields = make([]string, 0, len(fields))
		for _, field := range fields {
			if !field.PrimaryKey && !field.FormBuildExclude {
				formFields = append(formFields, field.Name)
			}
		}
	}
	columnFields := []string(nil)
	if raw.ColumnFields != nil {
		columnFields = *raw.ColumnFields
	} else {
		columnFields = make([]string, 0, len(fields))
		for _, field := range fields {
			columnFields = append(columnFields, field.Name)
		}
	}
	dataScope := &data_scope.Config{Mode: data_scope.ModeAuto}
	if raw.DataScope != nil {
		dataScope = &data_scope.Config{Mode: raw.DataScope.Mode, OwnerColumn: raw.DataScope.OwnerColumn, AssignOnCreate: raw.DataScope.AssignOnCreate}
	}
	typeName := raw.Type
	if typeName == "" {
		typeName = "create"
	}
	table := model.Table{
		Name: raw.Name, Comment: raw.Comment, FormFields: formFields, ColumnFields: columnFields,
		QuickSearchField: raw.QuickSearchField, DefaultSortField: raw.DefaultSortField, DefaultSortType: raw.DefaultSortType,
		ModelFile: raw.ModelFile, ControllerFile: raw.ControllerFile, WebViewsDir: raw.WebViewsDir,
		GenerateRelativePath: raw.GenerateRelativePath, DatabaseConnection: raw.DatabaseConnection,
		IsCommonModel: raw.IsCommonModel, Rebuild: raw.Rebuild, DataScope: dataScope,
	}
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

// normalizeNullKeys fixes YAML's special null key token before Viper sees it.
// Without this pass, an unquoted `null:` key can disappear during mapstructure
// decoding even though its value is a valid boolean.
func normalizeNullKeys(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			if key.Kind == yaml.ScalarNode && key.Tag == "!!null" && (key.Value == "null" || key.Value == "~") {
				key.Tag = "!!str"
				key.Value = "null"
			}
			if key.Value == "defaultType" && value.Kind == yaml.ScalarNode && value.Tag == "!!null" {
				value.Tag = "!!str"
				value.Value = "NULL"
			}
			if value.Kind == yaml.ScalarNode && value.Tag == "!!bool" && (key.Value == "selectMulti" || key.Value == "imageMulti" || key.Value == "fileMulti") {
				value.Tag = "!!str"
				if value.Value == "true" {
					value.Value = "1"
				} else {
					value.Value = ""
				}
			}
			if key.Value == "comSearchInputAttr" && value.Kind == yaml.ScalarNode {
				attrs, err := model.ParseComSearchInputAttrs(value.Value)
				if err != nil {
					return err
				}
				encoded, _ := yaml.Marshal(attrs)
				var replacement yaml.Node
				if yaml.Unmarshal(encoded, &replacement) == nil && len(replacement.Content) == 1 {
					*value = *replacement.Content[0]
				}
			}
			if err := normalizeNullKeys(value); err != nil {
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

// inferDesignTypeForField follows Helper.php::$inputTypeRule in order.  The
// order is significant: suffix rules intentionally beat broad type rules.
func inferDesignTypeForField(field model.Field) string {
	name := strings.ToLower(field.Name)
	typ := strings.ToLower(analyseFieldTypeForSpec(field))
	columnType := strings.ToLower(field.DataType)
	if columnType == "" {
		columnType = strings.ToLower(field.Type)
	}

	if field.AutoIncrement && strings.Contains(name, "id") {
		return "pk"
	}
	if name == "weigh" {
		return "weigh"
	}
	if isCanonicalTimeField(name) {
		return "timestamp"
	}
	if matchesTypeAndSuffix(typ, name, []string{"tinyint", "int", "enum"}, []string{"switch", "toggle"}) ||
		matchesColumnTypeAndSuffix(columnType, name, []string{"tinyint(1)", "char(1)", "tinyint(1) unsigned"}, []string{"switch", "toggle"}) {
		return "switch"
	}
	if matchesTypeAndSuffix(typ, name, []string{"longtext", "text", "mediumtext", "smalltext", "tinytext", "bigtext"}, []string{"content", "editor"}) {
		return "editor"
	}
	if matchesTypeAndSuffix(typ, name, []string{"varchar"}, []string{"textarea", "multiline", "rows"}) {
		return "textarea"
	}
	if hasSuffix(name, []string{"array"}) {
		return "array"
	}
	if matchesTypeAndSuffix(typ, name, []string{"int"}, []string{"time", "datetime"}) {
		return "timestamp"
	}
	switch typ {
	case "datetime", "timestamp":
		return "datetime"
	case "date":
		return "date"
	case "year":
		return "year"
	case "time":
		return "time"
	}
	if hasSuffix(name, []string{"select", "list", "data"}) {
		return "select"
	}
	if hasSuffix(name, []string{"selects", "multi", "lists"}) {
		return "selects"
	}
	if hasSuffix(name, []string{"_ids"}) {
		return "remoteSelects"
	}
	if hasSuffix(name, []string{"_id"}) {
		return "remoteSelect"
	}
	if hasSuffix(name, []string{"city"}) {
		return "city"
	}
	if hasSuffix(name, []string{"image", "avatar"}) {
		return "image"
	}
	if hasSuffix(name, []string{"images", "avatars"}) {
		return "images"
	}
	if hasSuffix(name, []string{"file"}) {
		return "file"
	}
	if hasSuffix(name, []string{"files"}) {
		return "files"
	}
	if hasSuffix(name, []string{"icon"}) {
		return "icon"
	}
	if matchesColumnTypeAndSuffix(columnType, name, []string{"tinyint(1)", "char(1)", "tinyint(1) unsigned"}, []string{"status", "state", "type"}) {
		return "radio"
	}
	if hasSuffix(name, []string{"number", "int", "num"}) {
		return "number"
	}
	if slicesContains([]string{"bigint", "int", "mediumint", "smallint", "tinyint", "decimal", "double", "float"}, typ) {
		return "number"
	}
	if slicesContains([]string{"longtext", "text", "mediumtext", "smalltext", "tinytext", "bigtext"}, typ) {
		return "textarea"
	}
	if typ == "enum" {
		return "radio"
	}
	if typ == "set" {
		return "checkbox"
	}
	if hasSuffix(name, []string{"color"}) {
		return "color"
	}
	return "string"
}

func analyseFieldTypeForSpec(field model.Field) string {
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

func applyTimestampTableDefaults(table *model.TableAttr) {
	if table.Render == "" {
		table.Render = "datetime"
	}
	if table.Operator == "" {
		table.Operator = "RANGE"
	}
	if table.ComSearchRender == "" {
		table.ComSearchRender = "datetime"
	}
	if table.Width == 0 {
		table.Width = 160
	}
	if table.TimeFormat == "" {
		table.TimeFormat = "yyyy-mm-dd hh:MM:ss"
	}
}

func applyDesignTypeDefaults(field *model.Field) {
	setTable := func(render, operator, sortable, search string, width int, timeFormat string) {
		if field.Table.Render == "" {
			field.Table.Render = render
		}
		if field.Table.Operator == "" {
			field.Table.Operator = operator
		}
		if field.Table.Sortable == "" {
			field.Table.Sortable = sortable
		}
		if field.Table.ComSearchRender == "" {
			field.Table.ComSearchRender = search
		}
		if field.Table.Width == 0 {
			field.Table.Width = width
		}
		if field.Table.TimeFormat == "" {
			field.Table.TimeFormat = timeFormat
		}
	}
	switch field.DesignType {
	case "pk":
		setTable("", "RANGE", "custom", "", 70, "")
	case "spk":
		setTable("", "RANGE", "custom", "", 180, "")
	case "weigh":
		setTable("", "RANGE", "custom", "", 0, "")
	case "timestamp":
		setTable("datetime", "RANGE", "custom", "datetime", 160, "yyyy-mm-dd hh:MM:ss")
		if len(field.Form.Validator) == 0 {
			field.Form.Validator = []string{"date"}
		}
	case "string":
		setTable("none", "LIKE", "false", "", 0, "")
	case "password":
		setTable("", "false", "", "", 0, "")
		if len(field.Form.Validator) == 0 {
			field.Form.Validator = []string{"password"}
		}
	case "number":
		setTable("none", "RANGE", "false", "", 0, "")
		if len(field.Form.Validator) == 0 {
			field.Form.Validator = []string{"number"}
		}
		if field.Form.Step == 0 {
			field.Form.Step = 1
		}
	case "float":
		setTable("none", "RANGE", "false", "", 0, "")
		if len(field.Form.Validator) == 0 {
			field.Form.Validator = []string{"float"}
		}
		if field.Form.Step == 0 {
			field.Form.Step = 1
		}
	case "radio":
		setTable("tag", "eq", "false", "", 0, "")
	case "checkbox":
		setTable("tags", "FIND_IN_SET", "false", "", 0, "")
	case "switch":
		setTable("switch", "eq", "false", "", 0, "")
	case "textarea":
		setTable("", "false", "", "", 0, "")
		if field.Form.Rows == 0 {
			field.Form.Rows = 3
		}
	case "array":
		setTable("", "false", "", "", 0, "")
	case "datetime":
		setTable("", "RANGE", "custom", "datetime", 160, "")
		if len(field.Form.Validator) == 0 {
			field.Form.Validator = []string{"date"}
		}
	case "year":
		setTable("", "RANGE", "custom", "", 0, "")
		if len(field.Form.Validator) == 0 {
			field.Form.Validator = []string{"date"}
		}
	case "date":
		setTable("", "RANGE", "custom", "date", 0, "")
		if len(field.Form.Validator) == 0 {
			field.Form.Validator = []string{"date"}
		}
	case "time":
		setTable("", "RANGE", "custom", "time", 0, "")
	case "select":
		setTable("tag", "eq", "false", "", 0, "")
	case "selects":
		setTable("tags", "FIND_IN_SET", "false", "", 0, "")
		if field.Form.SelectMulti == "" {
			field.Form.SelectMulti = "1"
		}
	case "remoteSelect":
		setTable("tags", "LIKE", "", "string", 0, "")
		if field.Form.RemotePk == "" {
			field.Form.RemotePk = "id"
		}
		if field.Form.RemoteField == "" {
			field.Form.RemoteField = "name"
		}
	case "remoteSelects":
		setTable("tags", "FIND_IN_SET", "", "remoteSelect", 0, "")
		if field.Form.SelectMulti == "" {
			field.Form.SelectMulti = "1"
		}
		if field.Form.RemotePk == "" {
			field.Form.RemotePk = "id"
		}
		if field.Form.RemoteField == "" {
			field.Form.RemoteField = "name"
		}
	case "editor":
		setTable("", "false", "", "", 0, "")
		if len(field.Form.Validator) == 0 {
			field.Form.Validator = []string{"editorRequired"}
		}
	case "city":
		setTable("", "false", "", "", 0, "")
	case "image":
		setTable("image", "false", "", "", 0, "")
	case "images":
		setTable("images", "false", "", "", 0, "")
		if field.Form.ImageMulti == "" {
			field.Form.ImageMulti = "1"
		}
	case "file":
		setTable("none", "false", "", "", 0, "")
	case "files":
		setTable("none", "false", "", "", 0, "")
		if field.Form.FileMulti == "" {
			field.Form.FileMulti = "1"
		}
	case "icon":
		setTable("icon", "false", "", "", 0, "")
	case "color":
		setTable("color", "false", "", "", 0, "")
	}
}
