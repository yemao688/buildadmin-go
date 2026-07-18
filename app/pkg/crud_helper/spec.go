package crud_helper

import (
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type specFile struct {
	Name             string         `mapstructure:"name"`
	Comment          string         `mapstructure:"comment"`
	Type             string         `mapstructure:"type"`
	ModelFile        string         `mapstructure:"modelFile"`
	ControllerFile   string         `mapstructure:"controllerFile"`
	WebViewsDir      string         `mapstructure:"webViewsDir"`
	IsCommonModel    int            `mapstructure:"isCommonModel"`
	Rebuild          string         `mapstructure:"rebuild"`
	QuickSearchField []string       `mapstructure:"quickSearchField"`
	DefaultSortField string         `mapstructure:"defaultSortField"`
	DefaultSortType  string         `mapstructure:"defaultSortType"`
	FormFields       []string       `mapstructure:"formFields"`
	ColumnFields     []string       `mapstructure:"columnFields"`
	DataScope        *specDataScope `mapstructure:"dataScope"`
	Fields           []specField    `mapstructure:"fields"`
	Menu             *specMenu      `mapstructure:"menu"`
}

type specDataScope struct {
	Mode           data_scope.Mode `mapstructure:"mode"`
	OwnerColumn    string          `mapstructure:"ownerColumn"`
	AssignOnCreate *bool           `mapstructure:"assignOnCreate"`
}

type specMenu struct {
	Title  string `mapstructure:"title"`
	Parent int32  `mapstructure:"parent"`
}

type specField struct {
	Name              string          `mapstructure:"name"`
	Title             string          `mapstructure:"title"`
	Type              string          `mapstructure:"type"`
	DataType          string          `mapstructure:"dataType"`
	Length            int             `mapstructure:"length"`
	Precision         int             `mapstructure:"precision"`
	Default           string          `mapstructure:"default"`
	Null              bool            `mapstructure:"null"`
	PrimaryKey        bool            `mapstructure:"primaryKey"`
	Unsigned          bool            `mapstructure:"unsigned"`
	AutoIncrement     bool            `mapstructure:"autoIncrement"`
	Comment           string          `mapstructure:"comment"`
	DesignType        string          `mapstructure:"designType"`
	Form              model.FormAttr  `mapstructure:"form"`
	Table             model.TableAttr `mapstructure:"table"`
	FormBuildExclude  bool            `mapstructure:"formBuildExclude"`
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
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")
	if err := v.ReadInConfig(); err != nil {
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
	allNames := make([]string, 0, len(raw.Fields))
	formNames := make([]string, 0, len(raw.Fields))
	for i, item := range raw.Fields {
		if item.Name == "" {
			return nil, fmt.Errorf("field[%d] name is required", i)
		}
		field := model.Field{
			Name: item.Name, Title: item.Title, Type: strings.ToLower(item.Type), DataType: strings.ToLower(item.DataType),
			Length: item.Length, Precision: item.Precision, Default: item.Default, Null: item.Null,
			PrimaryKey: item.PrimaryKey, Unsigned: item.Unsigned, AutoIncrement: item.AutoIncrement,
			Comment: item.Comment, DesignType: item.DesignType, Form: item.Form, Table: item.Table,
			FormBuildExclude: item.FormBuildExclude, TableBuildExclude: item.TableBuildExclude,
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
			field.DesignType = inferDesignType(field.Type)
		}
		if err := ValidateField(field); err != nil {
			return nil, fmt.Errorf("field %q: %w", item.Name, err)
		}
		fields = append(fields, field)
		allNames = append(allNames, field.Name)
		if !field.PrimaryKey {
			formNames = append(formNames, field.Name)
		}
	}

	formFields := raw.FormFields
	if len(formFields) == 0 {
		formFields = formNames
	}
	columnFields := raw.ColumnFields
	if len(columnFields) == 0 {
		columnFields = allNames
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
		IsCommonModel: raw.IsCommonModel, Rebuild: raw.Rebuild, DataScope: dataScope,
	}
	options := &GenerateOptions{Table: table, Fields: fields, Type: typeName}
	if raw.Menu != nil {
		options.Menu = &MenuOptions{Title: raw.Menu.Title, Parent: raw.Menu.Parent}
	}
	if err := ValidateGenerationInput(table, fields); err != nil {
		return nil, fmt.Errorf("spec %q validation failed: %w", path, err)
	}
	return options, nil
}

func inferDesignType(typ string) string {
	t := strings.ToLower(typ)
	if strings.Contains(t, "(") {
		t = t[:strings.IndexByte(t, '(')]
	}
	switch t {
	case "tinyint":
		return "number"
	case "int", "bigint", "smallint", "mediumint", "decimal", "double", "float", "real":
		return "number"
	case "datetime", "timestamp":
		return "datetime"
	case "date":
		return "date"
	case "time":
		return "time"
	case "enum", "set":
		return "select"
	case "text", "tinytext", "mediumtext", "longtext":
		return "textarea"
	default:
		return "string"
	}
}
