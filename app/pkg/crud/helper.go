package crud

import (
	"go-build-admin/app/admin/model"
)

// 内部保留词
var reservedKeywords = []string{
	"abstract", "and", "array", "as", "break", "callable", "case", "catch", "class", "clone",
	"const", "continue", "declare", "default", "die", "do", "echo", "else", "elseif", "empty",
	"enddeclare", "endfor", "endforeach", "endif", "endswitch", "endwhile", "eval", "exit", "extends",
	"final", "for", "foreach", "function", "global", "goto", "if", "implements", "include", "include_once",
	"instanceof", "insteadof", "interface", "isset", "list", "namespace", "new", "or", "print", "private",
	"protected", "public", "require", "require_once", "return", "static", "switch", "throw", "trait", "try",
	"unset", "use", "var", "while", "xor", "yield", "match", "readonly", "fn", "type",
}

//预设控制器和模型文件位置
// var parseNamePresets = []string{}

type Menu struct {
	Type   string
	Title  string
	Name   string
	Status string
}

// 子级菜单数组(权限节点)
var menuChildren = []Menu{
	{Type: "button", Title: "查看", Name: "/index", Status: "1"},
	{Type: "button", Title: "添加", Name: "/add", Status: "1"},
	{Type: "button", Title: "编辑", Name: "/edit", Status: "1"},
	{Type: "button", Title: "删除", Name: "/del", Status: "1"},
	{Type: "button", Title: "快速排序", Name: "/sortable", Status: "1"},
}

type InputRule struct {
	Type       []string
	Suffix     []string
	ColumnType []string
	Value      string
}

// 输入框类型的识别规则
var inputTypeRule = []InputRule{
	// 开关组件
	{
		Type:   []string{"tinyint", "int", "enum"},
		Suffix: []string{"switch", "toggle"},
		Value:  "switch",
	},
	{
		ColumnType: []string{"tinyint(1)", "char(1)", "tinyint(1) unsigned"},
		Suffix:     []string{"switch", "toggle"},
		Value:      "switch",
	},
	// 富文本-识别规则和textarea重合,优先识别为富文本
	{
		Type:   []string{"longtext", "text", "mediumtext", "smalltext", "tinytext", "bigtext"},
		Suffix: []string{"content", "editor"},
		Value:  "editor",
	},
	// textarea
	{
		Type:   []string{"varchar"},
		Suffix: []string{"textarea", "multiline", "rows"},
		Value:  "textarea",
	},
	// Array
	{
		Suffix: []string{"array"},
		Value:  "array",
	},
	// 时间选择器-字段类型为int同时以{"time", "datetime"}结尾
	{
		Type:   []string{"int"},
		Suffix: []string{"time", "datetime"},
		Value:  "timestamp",
	},
	{
		Type:  []string{"datetime", "timestamp"},
		Value: "datetime",
	},
	{
		Type:  []string{"date"},
		Value: "date",
	},
	{
		Type:  []string{"year"},
		Value: "year",
	},
	{
		Type:  []string{"time"},
		Value: "time",
	},
	// 单选select
	{
		Suffix: []string{"select", "list", "data"},
		Value:  "select",
	},
	// 多选select
	{
		Suffix: []string{"selects", "multi", "lists"},
		Value:  "selects",
	},
	// 远程select
	{
		Suffix: []string{"_id"},
		Value:  "remoteSelect",
	},
	// 远程selects
	{
		Suffix: []string{"_ids"},
		Value:  "remoteSelects",
	},
	// 城市选择器
	{
		Suffix: []string{"city"},
		Value:  "city",
	},
	// 单图上传
	{
		Suffix: []string{"image", "avatar"},
		Value:  "image",
	},
	// 多图上传
	{
		Suffix: []string{"images", "avatars"},
		Value:  "images",
	},
	// 文件上传
	{
		Suffix: []string{"file"},
		Value:  "file",
	},
	// 多文件上传
	{
		Suffix: []string{"files"},
		Value:  "files",
	},
	// icon选择器
	{
		Suffix: []string{"icon"},
		Value:  "icon",
	},
	// 单选框
	{
		ColumnType: []string{"tinyint(1)", "char(1)", "tinyint(1) unsigned"},
		Suffix:     []string{"status", "state", "type"},
		Value:      "radio",
	},
	// 数字输入框
	{
		Suffix: []string{"number", "int", "num"},
		Value:  "number",
	},
	{
		Type:  []string{"bigint", "int", "mediumint", "smallint", "tinyint", "decimal", "double", "float"},
		Value: "number",
	},
	// 富文本-低权重
	{
		Type:  []string{"longtext", "text", "mediumtext", "smalltext", "tinytext", "bigtext"},
		Value: "textarea",
	},
	// 单选框-低权重
	{
		Type:  []string{"enum"},
		Value: "radio",
	},
	// 多选框
	{
		Type:  []string{"set"},
		Value: "checkbox",
	},
	// 颜色选择器
	{
		Suffix: []string{"color"},
		Value:  "color",
	},
}

//预设WEB端文件位置
// var parseWebDirPresets = []string{}

var createTimeField = "create_time"
var updateTimeField = "update_time"

// 属性的类型对照表
var attrType = []string{}

// 获取字段字典数据
func GetDictData(dict map[string]any) {

}

func getPhinxFieldType() {

}

func analyseFieldLimit() {

}

func dataTypeLimit() {

}

func analyseFieldDefault() {

}

func searchArray() {

}

func getPhinxFieldData() {

}

func updateFieldOrder() {

}

func HandleTableDesign(table model.JSON_TABLE, fields []model.Field) {

}

type NameInfo struct {
	LastName         string
	OriginalLastName string
	Path             []string
	Namespace        string
	ParseFile        string
	RootFileName     string
}

func ParseNameData(app string, tableName string, t string, value string) NameInfo {
	info := NameInfo{
		LastName:         "",
		OriginalLastName: "",
		Path:             []string{},
		Namespace:        "",
		ParseFile:        "",
		RootFileName:     "",
	}
	return info
}

type WebDir struct {
	Path             string
	LastName         string
	OriginalLastName string
	Lang             string
	Views            string
}

func ParseWebDirNameData(tableName string, t string, value string) WebDir {
	webDir := WebDir{
		Path:             "",
		LastName:         "",
		OriginalLastName: "",
		Lang:             "",
		Views:            "",
	}
	return webDir
}

func getMenuName() {

}

func getStubFilePath() {

}

func assembleStub() {

}

func escape() {

}

func tab() {

}

func parseTableColumns() {

}

func handleTableColumn() {

}

func analyseFieldType() {

}

func analyseFieldDataType() {

}

func analyseField() {

}

func getTableColumnsDataType() {

}

func isMatchSuffix() {

}

func createMenu() {

}

func writeWebLangFile() {

}

func writeFile() {

}

func buildModelAppend() {

}

func buildModelFieldType() {

}

func writeModelFile() {

}

func writeControllerFile() {

}

func writeFormFile() {

}

func buildFormValidatorRules() {

}

func writeIndexFile() {

}

func buildTableColumn() {

}

func buildTableColumnKey() {

}

func formatObjectKey() {

}

func getQuote() {

}

func buildFormatSimpleArray() {

}

func buildSimpleArray() {

}

func buildDefaultOrder() {

}

func getJsonFromArray() {

}
