package crudgen

import (
	"reflect"
	"strings"
)

// specSchemaDescriptions 是 schema 里的人工描述；键从结构体 tag 反射生成，
// 因此新增 spec 键时只要改结构体，描述缺省也不影响可用性。
var specSchemaDescriptions = map[string]string{
	"name":                 "业务表名，小写下划线标识符",
	"comment":              "表注释；字段标题与字典也来自注释",
	"type":                 "auto（默认，按库现状推导）| create（删表重建）| alter（只补/改列）| log/db/sql（legacy，映射为 create/alter）",
	"generateRelativePath": "views 目录与菜单/路由名形态；省略即表名",
	"quickSearchField":     "列表公共快速搜索字段，支持 alias.column 关联条目",
	"defaultSortField":     "默认排序字段",
	"defaultSortType":      "默认排序方向 asc|desc",
	"formFields":           "表单项字段；省略自动推导，[] 表示无表单项",
	"columnFields":         "列表列字段；省略自动推导，建议显式设置",
	"dataScope":            "数据权限策略：mode/ownerColumn/assignOnCreate/readExtraOwners/reassignable/inheritFrom",
	"menu":                 "菜单标题与父节点覆盖：title/parent/weigh",
	"indexes":              "表级索引：name/unique/columns",
	"fields":               "字段定义；序列写法 [- name: x] 或映射写法 {x: {type: varchar}}",
	"webViewsDir":          "views 路径覆盖，默认 web/src/views/backend/<relativePath>",
	"registerOnly":         "仅登记不生成，保留给受保护核心表",

	"title":             "字段标题，默认由注释推导",
	"dataType":          "SQL 类型；与 type 二选一",
	"length":            "长度（varchar/char/decimal 等）",
	"precision":         "小数位（decimal）",
	"default":           "默认值字面量；与 defaultType 配对",
	"defaultType":       "NULL | EMPTY STRING | NONE | INPUT，省略时按 default 推导",
	"null":              "是否允许 NULL，默认 false",
	"primaryKey":        "是否主键；每个 spec 恰好一个",
	"unsigned":          "是否 unsigned",
	"autoIncrement":     "是否自增",
	"designType":        "表单组件类型，如 switch/select/remoteSelect/editor/image",
	"form":              "表单属性：validator/render/remoteTable/remoteField 等",
	"table":             "列表属性：render/width/operator/sortable 等",
	"formBuildExclude":  "是否从表单排除",
	"tableBuildExclude": "是否从列表排除",
}

// SpecSchema 返回 crud spec 的 JSON Schema（draft-07）。键与类型由 specFile /
// specField 的 mapstructure tag 反射生成，避免新增键后 schema 漂移；描述来自
// specSchemaDescriptions。字段既支持序列写法也支持映射写法，因此 fields 用
// oneOf 表达。
func SpecSchema() map[string]any {
	root := structSchema(reflect.TypeOf(specFile{}))
	properties, _ := root["properties"].(map[string]any)
	if properties == nil {
		properties = map[string]any{}
	}
	properties["type"] = map[string]any{
		"type":        "string",
		"enum":        []string{"auto", "create", "alter", "log", "db", "sql"},
		"default":     "auto",
		"description": specSchemaDescriptions["type"],
	}

	fieldProperties := structSchema(reflect.TypeOf(specField{}))["properties"]
	fieldItem := map[string]any{
		"type":       "object",
		"properties": fieldProperties,
		"required":   []string{"name"},
	}
	properties["fields"] = map[string]any{
		"description": specSchemaDescriptions["fields"],
		"oneOf": []any{
			map[string]any{"type": "array", "items": fieldItem, "minItems": 1},
			map[string]any{"type": "object", "additionalProperties": fieldItem, "minProperties": 1},
		},
	}
	root["$schema"] = "http://json-schema.org/draft-07/schema#"
	root["title"] = "BuildAdmin-Go CRUD spec"
	root["required"] = []string{"name", "fields"}
	return root
}

// structSchema 把结构体反射成 JSON Schema 片段；mapstructure 为空的字段视为
// 内部字段跳过，嵌套业务结构统一用宽松 object（其契约由校验器负责）。
func structSchema(typ reflect.Type) map[string]any {
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	properties := make(map[string]any, typ.NumField())
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		key := strings.Split(field.Tag.Get("mapstructure"), ",")[0]
		if key == "" {
			continue
		}
		properties[key] = typeSchema(field.Type, key)
	}
	return map[string]any{"type": "object", "properties": properties}
}

func typeSchema(typ reflect.Type, key string) map[string]any {
	schema := map[string]any{}
	if description, ok := specSchemaDescriptions[key]; ok {
		schema["description"] = description
	}
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.String:
		schema["type"] = "string"
	case reflect.Bool:
		schema["type"] = "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schema["type"] = "integer"
	case reflect.Float32, reflect.Float64:
		schema["type"] = "number"
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.String {
			schema["type"] = "array"
			schema["items"] = map[string]any{"type": "string"}
		} else {
			schema["type"] = "array"
			schema["items"] = structSchema(typ.Elem())
		}
	case reflect.Map, reflect.Struct:
		schema["type"] = "object"
		schema["additionalProperties"] = true
	default:
		schema["type"] = "object"
		schema["additionalProperties"] = true
	}
	return schema
}
