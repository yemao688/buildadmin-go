package crud

import (
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 内部保留词
var reservedKeywords = []string{
	"abstract", "and", "array", "as", "break", "callable", "case", "catch", "class", "clone",
	"const", "continue", "declare", "default", "die", "do", "echo", "else", "elseif", "empty",
	"enddeclare", "endfor", "endforeach", "endif", "endswitch", "endwhile", "eval", "exit", "extends",
	"final", "for", "foreach", "function", "global", "goto", "if", "implements", "include", "include_once",
	"instanceof", "insteadof", "interface", "isset", "list", "namespace", "new", "or", "print", "private",
	"protected", "public", "require", "require_once", "return", "static", "switch", "throw", "trait", "try",
	"unset", "use", "var", "while", "xor", "yield", "match", "readonly", "fn",
}

//预设控制器和模型文件位置
// var parseNamePresets = []string{}

//预设WEB端文件位置
// var parseWebDirPresets = []string{}

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

var createTimeField = "create_time"
var updateTimeField = "update_time"

var attrType = []string{}

const TableNameCrudLog = "ba_crud_log"

type CrudLog struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                           // ID
	Tablename  string `gorm:"column:table_name;not null;comment:数据表名" json:"table_name"`                                              // 数据表名
	Table      string `gorm:"column:table;comment:数据表数据" json:"table"`                                                                // 数据表数据
	Fields     string `gorm:"column:fields;comment:字段数据" json:"fields"`                                                               // 字段数据
	Status     string `gorm:"column:status;not null;default:start;comment:状态:delete=已删除,success=成功,error=失败,start=生成中" json:"status"` // 状态:delete=已删除,success=成功,error=失败,start=生成中
	CreateTime int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`                                                     // 创建时间
}

type Helper struct {
	log   *zap.Logger
	sqlDB *gorm.DB
}

func NewHelper(log *zap.Logger, sqlDB *gorm.DB) *Helper {
	return &Helper{
		log:   log,
		sqlDB: sqlDB,
	}
}

// 获取字段字典数据
func (h *Helper) getDictData() {

}

// 记录CRUD状态
func (h *Helper) recordCrudStatus(data CrudLog) int32 {
	if data.ID != 0 {
		h.sqlDB.Table(TableNameCrudLog).Where("id=?", data.ID).Update("status", data.Status)
		return data.ID
	}

	h.sqlDB.Table(TableNameCrudLog).Where("id=?", data.ID).Update("status", data.Status)
	return data.ID
}

func (h *Helper) getPhinxFieldType() {

}

func (h *Helper) analyseFieldLimit() {

}

func (h *Helper) dataTypeLimit() {

}

func (h *Helper) analyseFieldDefault() {

}

func (h *Helper) searchArray() {

}

func (h *Helper) getPhinxFieldData() {

}

func (h *Helper) updateFieldOrder() {

}

func (h *Helper) handleTableDesign() {

}

func (h *Helper) parseNameData() {

}

func (h *Helper) parseWebDirNameData() {

}

func (h *Helper) getMenuName() {

}

func (h *Helper) getStubFilePath() {

}

func (h *Helper) assembleStub() {

}

func (h *Helper) escape() {

}

func (h *Helper) tab() {

}

func (h *Helper) delTable() {

}

func (h *Helper) parseTableColumns() {

}

func (h *Helper) handleTableColumn() {

}

func (h *Helper) analyseFieldType() {

}

func (h *Helper) analyseFieldDataType() {

}

func (h *Helper) analyseField() {

}

func (h *Helper) getTableColumnsDataType() {

}

func (h *Helper) isMatchSuffix() {

}

func (h *Helper) createMenu() {

}

func (h *Helper) writeWebLangFile() {

}

func (h *Helper) writeFile() {

}

func (h *Helper) buildModelAppend() {

}

func (h *Helper) buildModelFieldType() {

}

func (h *Helper) writeModelFile() {

}

func (h *Helper) writeControllerFile() {

}

func (h *Helper) writeFormFile() {

}

func (h *Helper) buildFormValidatorRules() {

}

func (h *Helper) writeIndexFile() {

}

func (h *Helper) buildTableColumn() {

}

func (h *Helper) buildTableColumnKey() {

}

func (h *Helper) formatObjectKey() {

}

func (h *Helper) getQuote() {

}

func (h *Helper) buildFormatSimpleArray() {

}

func (h *Helper) buildSimpleArray() {

}

func (h *Helper) buildDefaultOrder() {

}

func (h *Helper) getJsonFromArray() {

}
