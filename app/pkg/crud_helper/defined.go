package crud_helper

// 内部保留词
var reservedKeywords = []string{
	"abstract", "and", "array", "as", "break", "callable", "case", "catch", "class", "clone",
	"const", "continue", "declare", "default", "die", "do", "echo", "else", "elseif", "empty",
	"enddeclare", "endfor", "endforeach", "endif", "endswitch", "endwhile", "eval", "exit", "extends",
	"final", "for", "foreach", "function", "global", "goto", "if", "implements", "include", "include_once",
	"instanceof", "insteadof", "interface", "isset", "list", "namespace", "new", "or", "print", "private",
	"protected", "public", "require", "require_once", "return", "static", "switch", "throw", "trait", "try",
	"unset", "use", "var", "while", "xor", "yield", "match", "readonly", "fn", "type", "go",
}

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

// 预设WEB端文件位置
var parseWebDirPresets = map[string][]string{
	"views/user":        {"user", "user"},
	"views/admin":       {"auth", "admin"},
	"views/admin_group": {"auth", "group"},
	"views/attachment":  {"routine", "attachment"},
	"views/admin_rule":  {"auth", "rule"},
}

type IndexVueData struct {
	EnableDragSort        string
	DefaultItems          []string
	TableColumn           []string
	DblClickNotEditColumn []string
	OptButtons            []string
	DefaultOrder          string
	TablePk               string
	WebTranslate          string
}

type FormVueData struct {
	BigDialog  string
	FormFields []string
}

type WebDir struct {
	OriginalLastName string
	LastName         string
	Path             []string
	Views            string
	Lang             []string
	LangDir          string
}

// 当designType为以下值时: 1. 出入库字符串到数组转换,2. 默认值转数组
var dtStringToArray = []string{"checkbox", "selects", "remoteSelects", "city", "images", "files"}

type GetTableName func(string, bool) string
type GetColumns func(string) ([]map[string]string, error)

// 预设控制器和模型文件位置
var parseNamePresets = map[string][]string{
	"handler/user":        {"user"},
	"handler/admin":       {"admin"},
	"handler/admin_group": {"admin_group"},
	"handler/attachment":  {"attachment"},
	"handler/admin_rule":  {"admin_rule"},
}

type NameInfo struct {
	LastName         string
	OriginalLastName string
	Path             []string
	Namespace        string
	ParseFile        string
	RootFileName     string
}

// 属性的类型对照表
var attrType = map[string]string{
	"handler/preExcludeFields": "string",
	"handler/quickSearchField": "string",
	"handler/withJoinTable":    "array",
	"handler/defaultSortField": "string",
}

var createTimeField = "create_time"
var updateTimeField = "update_time"

type HandlerData struct {
	Namespace      string //包名
	ClassName      string //类名
	ModelNamespace string //模型包名
	ModelName      string //模型类名
	ModelVar       string //模型变量名
	TableComment   string //表备注

	Import     []string //需要引入的包名
	FilterRule []string //对前端数据进行过滤方法

	Attr                     map[string]string // preExcludeFields quickSearchField withJoinTable defaultSortField
	Methods                  []string
	RelationVisibleFieldList map[string][]string
}

const handlerTemp = `
package {{.Namespace}}

import (
	"go-build-admin/app/admin/{{.ModelNamespace}}"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type {{.ClassName}}Handler struct {
	Base
	log        *zap.Logger
	{{.ModelVar}} *model.{{.ModelName}}Model
}

func New{{.ClassName}}Handler(log *zap.Logger, {{.ModelVar}} *model.{{.ModelName}}Model) *{{.ClassName}}Handler {
	return &{{.ClassName}}Handler{Base: Base{currentM: {{.ModelVar}}}, log: log, {{.ModelVar}}: {{.ModelVar}}}
}

func (h *{{.ClassName}}Handler) Index(ctx *gin.Context) {
	list, total, err := h.{{.ModelVar}}.List(ctx)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, map[string]any{
		"list":   list,
		"total":  total,
		"remark": "",
	})
}

type {{.ClassName}}Param struct {
}

func (h *{{.ClassName}}Handler) Add(ctx *gin.Context) {
	var params {{.ClassName}}Param
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.{{.ClassName}}
	copier.Copy(&data, params)
	err := h.{{.ModelVar}}.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *{{.ClassName}}Handler) Edit(ctx *gin.Context) {
	var params {{.ClassName}}Param
	if err := ctx.ShouldBindQuery(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	var data model.{{.ClassName}}
	copier.Copy(&data, params)
	err := h.{{.ModelVar}}.Edit(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *{{.ClassName}}Handler) Del(ctx *gin.Context) {
	var param validate.Ids
	if err := ctx.ShouldBindQuery(&param); err != nil {
		FailByErr(ctx, validate.GetError(param, err))
		return
	}
	err := h.{{.ModelVar}}.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
`

type ModelData struct {
	Namespace    string //包名
	Name         string //表名
	ClassName    string //类名
	TableComment string //表备注
	StructTemp   string //结构体
	Pk           string //主键
	ModelVar     string //结构体变量

	Append             []string
	Methods            []string
	FieldType          map[string]string
	CreateTime         string
	UpdateTime         string
	AutoWriteTimestamp string
	BeforeInsertMixins map[string]string
	BeforeInsert       string
	AfterInsert        string
	RelationMethodList map[string]string
}

const modelTemp = `
package {{.Namespace}}

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableName{{.ClassName}} = "{{.Name}}"

// {{.ClassName}} {{.TableComment}}
{{.StructTemp}}

func (*{{.ClassName}}) TableName() string {
	return TableName{{.ClassName}}
}

type {{.ClassName}}Model struct {
	BaseModel
}

func New{{.ClassName}}Model(sqlDB *gorm.DB) *{{.ClassName}}Model {
	return &{{.ClassName}}Model{
		BaseModel: BaseModel{
			TableName:        TableName{{.ClassName}},
			Key:              {{.Pk}},
			QuickSearchField: "name",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *{{.ClassName}}Model) GetOne(ctx *gin.Context, id int32) ({{.ModelVar}} {{.ClassName}}, err error) {
	err = s.sqlDB.Table(s.TableName).Where("id=?", id).First(&{{.ModelVar}}).Error
	return
}

func (s *{{.ClassName}}Model) List(ctx *gin.Context) (list []{{.ClassName}}, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, err
	}
	err = s.sqlDB.Table(s.TableName).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *{{.ClassName}}Model) Add(ctx *gin.Context, {{.ModelVar}} {{.ClassName}}) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table(s.TableName).Create(&{{.ModelVar}}).Error; err != nil {
		tx.Rollback()
		return err

	}
	return tx.Commit().Error
}

func (s *{{.ClassName}}Model) Edit(ctx *gin.Context, {{.ModelVar}} {{.ClassName}}) error {
	tx := s.sqlDB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Table(s.TableName).Save(&{{.ModelVar}}).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (s *{{.ClassName}}Model) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
`
