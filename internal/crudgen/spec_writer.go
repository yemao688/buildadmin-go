package crudgen

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// specDocument 是写回 crud_specs 的文档形状：字段顺序固定，便于 review 与 diff；
// 只输出非零值，避免把生成器的默认值固化成显式配置。
//
// type 刻意不写：设计器的「创建」语义只作用于本次请求，写进 spec 会让后续
// CLI 重新生成误删表；省略即 auto，按库现状推导。
type specDocument struct {
	Name                 string           `yaml:"name"`
	Comment              string           `yaml:"comment,omitempty"`
	GenerateRelativePath string           `yaml:"generateRelativePath,omitempty"`
	QuickSearchField     []string         `yaml:"quickSearchField,omitempty"`
	DefaultSortField     string           `yaml:"defaultSortField,omitempty"`
	DefaultSortType      string           `yaml:"defaultSortType,omitempty"`
	FormFields           *[]string        `yaml:"formFields,omitempty"`
	ColumnFields         *[]string        `yaml:"columnFields,omitempty"`
	DataScope            map[string]any   `yaml:"dataScope,omitempty"`
	Indexes              []map[string]any `yaml:"indexes,omitempty"`
	Fields               []specFieldDoc   `yaml:"fields"`
}

type specFieldDoc struct {
	Name              string         `yaml:"name"`
	Title             string         `yaml:"title,omitempty"`
	Type              string         `yaml:"type,omitempty"`
	DataType          string         `yaml:"dataType,omitempty"`
	Length            int            `yaml:"length,omitempty"`
	Precision         int            `yaml:"precision,omitempty"`
	Default           string         `yaml:"default,omitempty"`
	DefaultType       string         `yaml:"defaultType,omitempty"`
	Null              bool           `yaml:"null,omitempty"`
	PrimaryKey        bool           `yaml:"primaryKey,omitempty"`
	Unsigned          bool           `yaml:"unsigned,omitempty"`
	AutoIncrement     bool           `yaml:"autoIncrement,omitempty"`
	Comment           string         `yaml:"comment,omitempty"`
	DesignType        string         `yaml:"designType,omitempty"`
	Form              map[string]any `yaml:"form,omitempty"`
	Table             map[string]any `yaml:"table,omitempty"`
	FormBuildExclude  *bool          `yaml:"formBuildExclude,omitempty"`
	TableBuildExclude bool           `yaml:"tableBuildExclude,omitempty"`
}

// MarshalSpec 把设计器提交的表设计序列化为 spec YAML。产物必须能被 LoadSpec
// 原样读回（测试守住往返一致），否则设计器与 CLI 会再次分叉。
func MarshalSpec(table crudmodel.Table, fields []crudmodel.Field) ([]byte, error) {
	document := specDocument{
		Name:                 table.Name,
		Comment:              table.Comment,
		GenerateRelativePath: table.GenerateRelativePath,
		QuickSearchField:     table.QuickSearchField,
		DefaultSortField:     table.DefaultSortField,
		DefaultSortType:      table.DefaultSortType,
		FormFields:           stringSlicePointer(table.FormFields),
		ColumnFields:         stringSlicePointer(table.ColumnFields),
	}
	if table.DataScope != nil {
		scope, err := structToMap(table.DataScope)
		if err != nil {
			return nil, fmt.Errorf("encode dataScope: %w", err)
		}
		document.DataScope = scope
	}
	for _, index := range table.Indexes {
		item, err := structToMap(index)
		if err != nil {
			return nil, fmt.Errorf("encode index %q: %w", index.Name, err)
		}
		document.Indexes = append(document.Indexes, item)
	}
	for _, field := range fields {
		item, err := buildSpecFieldDoc(field)
		if err != nil {
			return nil, err
		}
		document.Fields = append(document.Fields, item)
	}
	body, err := yaml.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode spec: %w", err)
	}
	header := "# 由 crudgen 生成（后台设计器或 crud:export-spec）；改动后请用 crud:validate / crud:generate\n"
	return append([]byte(header), body...), nil
}

func buildSpecFieldDoc(field crudmodel.Field) (specFieldDoc, error) {
	document := specFieldDoc{
		Name: field.Name, Title: field.Title, Type: field.Type, DataType: field.DataType,
		Length: field.Length, Precision: field.Precision, Default: field.Default, DefaultType: field.DefaultType,
		Null: field.Null, PrimaryKey: field.PrimaryKey, Unsigned: field.Unsigned, AutoIncrement: field.AutoIncrement,
		Comment: field.Comment, DesignType: field.DesignType, TableBuildExclude: field.TableBuildExclude,
	}
	if field.FormBuildExclude {
		excluded := true
		document.FormBuildExclude = &excluded
	}
	form, err := structToMap(field.Form)
	if err != nil {
		return specFieldDoc{}, fmt.Errorf("encode field %q form: %w", field.Name, err)
	}
	if len(form) > 0 {
		document.Form = form
	}
	table, err := structToMap(field.Table)
	if err != nil {
		return specFieldDoc{}, fmt.Errorf("encode field %q table: %w", field.Name, err)
	}
	if len(table) > 0 {
		document.Table = table
	}
	return document, nil
}

// structToMap 按 mapstructure tag（缺省用字段名）把属性结构体转成通用 map，
// 保证写出的键能被 LoadSpec 原样读回；零值不落盘。不能用 JSON tag：表单属性
// 的 JSON 键是 remote-pk 这类连字符形式，与 YAML 契约（remotePk）不同。
func structToMap(value any) (map[string]any, error) {
	converted, empty, err := toSpecValue(reflect.ValueOf(value))
	if err != nil {
		return nil, err
	}
	if empty {
		return map[string]any{}, nil
	}
	result, ok := converted.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected a struct, got %T", value)
	}
	return result, nil
}

// toSpecValue 转换单个属性值并报告是否为零值（零值不写入 spec）。
func toSpecValue(value reflect.Value) (any, bool, error) {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, true, nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.String:
		text := value.String()
		return text, text == "", nil
	case reflect.Bool:
		flag := value.Bool()
		return flag, !flag, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		number := value.Int()
		return number, number == 0, nil
	case reflect.Float32, reflect.Float64:
		number := value.Float()
		return number, number == 0, nil
	case reflect.Slice:
		if value.IsNil() || value.Len() == 0 {
			return nil, true, nil
		}
		items := make([]any, 0, value.Len())
		for index := 0; index < value.Len(); index++ {
			item, empty, err := toSpecValue(value.Index(index))
			if err != nil {
				return nil, false, err
			}
			if empty {
				continue
			}
			items = append(items, item)
		}
		if len(items) == 0 {
			return nil, true, nil
		}
		return items, false, nil
	case reflect.Map:
		if value.IsNil() || value.Len() == 0 {
			return nil, true, nil
		}
		result := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			key, ok := iter.Key().Interface().(string)
			if !ok {
				return nil, false, fmt.Errorf("spec attribute map keys must be strings")
			}
			item, empty, err := toSpecValue(iter.Value())
			if err != nil {
				return nil, false, err
			}
			if empty {
				continue
			}
			result[key] = item
		}
		if len(result) == 0 {
			return nil, true, nil
		}
		return result, false, nil
	case reflect.Struct:
		result := make(map[string]any, value.NumField())
		typ := value.Type()
		for index := 0; index < typ.NumField(); index++ {
			field := typ.Field(index)
			if field.PkgPath != "" {
				continue
			}
			key := strings.Split(field.Tag.Get("mapstructure"), ",")[0]
			if key == "" || key == "-" {
				// mapstructure 缺省按字段名大小写不敏感匹配，写字段名即可。
				key = field.Name
			}
			item, empty, err := toSpecValue(value.Field(index))
			if err != nil {
				return nil, false, err
			}
			if empty {
				continue
			}
			result[key] = item
		}
		if len(result) == 0 {
			return nil, true, nil
		}
		return result, false, nil
	default:
		return nil, false, fmt.Errorf("unsupported spec attribute kind %s", value.Kind())
	}
}

// stringSlicePointer 区分「省略」（nil）与「显式空列表」：表单字段为空列表
// 表示无表单项，必须原样写回 []。
func stringSlicePointer(values []string) *[]string {
	if values == nil {
		return nil
	}
	copied := append([]string{}, values...)
	return &copied
}
