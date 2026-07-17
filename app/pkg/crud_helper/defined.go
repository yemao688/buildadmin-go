package crud_helper

import (
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
)

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
type GetColumns func(string) ([]model.Column, error)

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
// var attrType = map[string]string{
// 	"handler/preExcludeFields": "string",
// 	"handler/quickSearchField": "string",
// 	"handler/withJoinTable":    "array",
// 	"handler/defaultSortField": "string",
// }

// var createTimeField = "create_time"
// var updateTimeField = "update_time"

type HandlerData struct {
	Namespace       string //包名
	ClassName       string //类名
	ModelNamespace  string //模型包名
	ModelImportPath string //模型完整导入路径
	ModelName       string //模型类名
	ModelVar        string //模型变量名
	TableComment    string //表备注
	ValidateParam   string //表单参数

	Import     []string //需要引入的包名
	FilterRule []string //对前端数据进行过滤方法

	Attr                     map[string]string // preExcludeFields quickSearchField withJoinTable defaultSortField
	Methods                  []string
	RelationVisibleFieldList map[string][]string

	ExcludeParamFields []string // fields that must not appear in Add/Edit DTO
}

const handlerTemp = `
package {{.Namespace}}

import (
	"{{.ModelImportPath}}"
	"go-build-admin/app/admin/validate"
	"go-build-admin/app/pkg/validator"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"go.uber.org/zap"
)

type {{.ClassName}}Handler struct {
	Base
	log        *zap.Logger
	{{.ModelVar}}M *model.{{.ModelName}}Model
}

func New{{.ClassName}}Handler(log *zap.Logger, {{.ModelVar}}M *model.{{.ModelName}}Model) *{{.ClassName}}Handler {
	return &{{.ClassName}}Handler{Base: Base{currentM: {{.ModelVar}}M}, log: log, {{.ModelVar}}M: {{.ModelVar}}M}
}

func (h *{{.ClassName}}Handler) Index(ctx *gin.Context) {
	if data, ok := h.Select(ctx); ok {
		Success(ctx, data)
	}
	list, total, err := h.{{.ModelVar}}M.List(ctx)
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

{{.ValidateParam}}


func (h *{{.ClassName}}Handler) Add(ctx *gin.Context) {
	var params {{.ClassName}}Param
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}
	var data model.{{.ClassName}}
	copier.Copy(&data, params)
	err := h.{{.ModelVar}}M.Add(ctx, data)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}

func (h *{{.ClassName}}Handler) Edit(ctx *gin.Context) {
	if h.MaybePartialEdit(ctx, map[string]bool{"status": true}) {
		return
	}

	var params = struct {
		IDS
		{{.ClassName}}Param
	}{}
	if err := ctx.ShouldBindJSON(&params); err != nil {
		FailByErr(ctx, validator.GetError(params, err))
		return
	}

	data, err := h.{{.ModelVar}}M.GetOne(ctx, params.ID)
	if err != nil {
		FailByErr(ctx, err)
		return
	}

	copier.Copy(&data, params)
	err = h.{{.ModelVar}}M.Edit(ctx, data)
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
	err := h.{{.ModelVar}}M.Del(ctx, param.Ids)
	if err != nil {
		FailByErr(ctx, err)
		return
	}
	Success(ctx, "")
}
`

type ModelData struct {
	Namespace  string //包名
	Name       string //表名
	ClassName  string //类名
	Pk         string //主键
	PkGoField  string //主键Go字段名
	ModelVar   string //结构体变量
	StructTemp string //结构体

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

	DataScopePolicy       data_scope.ResourcePolicy
	DataScopeOwnerGoField string
	EffectiveFormFields   []string
	EditableColumns       []string
	EditableColumnsGo     string
}

const modelTemp = `package {{.Namespace}}

import (
	"fmt"
	"go-build-admin/app/pkg/data_scope"
)

{{.StructTemp}}

type {{.ClassName}}Model struct {
	BaseModel
	Policy   data_scope.ResourcePolicy
	Enforcer data_scope.Enforcer
}

func New{{.ClassName}}Model(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *{{.ClassName}}Model {
	return &{{.ClassName}}Model{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "{{.Name}}",
			Key:              "{{.Pk}}",
			QuickSearchField: "name",
			sqlDB:            sqlDB,
		},
		Policy: data_scope.ResourcePolicy{
			Mode:           "{{.DataScopePolicy.Mode}}",
			OwnerColumn:    "{{.DataScopePolicy.OwnerColumn}}",
			AssignOnCreate: {{.DataScopePolicy.AssignOnCreate}},
		},
		Enforcer: enforcer,
	}
}

func (s *{{.ClassName}}Model) scopedDB(ctx *gin.Context) *gorm.DB {
	return s.scopeDB(ctx, s.DBFor(ctx))
}

func (s *{{.ClassName}}Model) scopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	if s.Policy.Mode == data_scope.ModeNone {
		return db
	}
	if s.Enforcer == nil {
		tx := db.Session(&gorm.Session{})
		_ = tx.AddError(data_scope.ErrScopedAccessDenied)
		return tx
	}
	return s.Enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: s.Policy.OwnerColumn})
}

// ScopeDB exposes the generated model's data-scope application to generic CRUD handlers.
func (s *{{.ClassName}}Model) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	return s.scopeDB(ctx, db)
}

func (s *{{.ClassName}}Model) GetOne(ctx *gin.Context, id int32) ({{.ModelVar}} {{.ClassName}}, err error) {
	db := s.scopedDB(ctx).Session(&gorm.Session{})
	db.Statement.Table = s.TableName
	err = db.Where("id=?", id).First(&{{.ModelVar}}).Error
	return
}

func (s *{{.ClassName}}Model) List(ctx *gin.Context) (list []{{.ClassName}}, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	countDB := s.scopedDB(ctx).Session(&gorm.Session{})
	countDB.Statement.Table = s.TableName
	countDB = countDB.Where(whereS, whereP...)
	if err = countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	findDB := s.scopedDB(ctx).Session(&gorm.Session{})
	findDB.Statement.Table = s.TableName
	findDB = findDB.Where(whereS, whereP...)
	err = findDB.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *{{.ClassName}}Model) Add(ctx *gin.Context, {{.ModelVar}} {{.ClassName}}) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		{{if .DataScopePolicy.AssignOnCreate}}actor, err := s.Enforcer.Actor(ctx)
		if err != nil {
			return err
		}
		if s.Policy.AssignOnCreate {
			{{.ModelVar}}.{{.DataScopeOwnerGoField}} = actor.AdminID
		}
		{{else}}if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}{{end}}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		return tx.Table(s.TableName).Create(&{{.ModelVar}}).Error
	})
}

func (s *{{.ClassName}}Model) Edit(ctx *gin.Context, {{.ModelVar}} {{.ClassName}}) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		tx = s.scopeDB(ctx, tx)

	res := tx.Table(s.TableName).Model(&{{.ModelVar}}).Where("{{.Pk}} = ?", {{.ModelVar}}.{{.PkGoField}}).Select({{.EditableColumnsGo}}).Updates(&{{.ModelVar}})
	if err := res.Error; err != nil {
		return err
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
	})
}

func (s *{{.ClassName}}Model) Del(ctx *gin.Context, ids interface{}) error {
	normalizedIDs, err := normalize{{.ClassName}}IDs(ids)
	if err != nil {
		return err
	}
	if len(normalizedIDs) == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		tx = s.scopeDB(ctx, tx)

	var visible int64
	if err := tx.Table(s.TableName).Model(&{{.ClassName}}{}).Where("{{.Pk}} IN ?", normalizedIDs).Count(&visible).Error; err != nil {
		return err
	}
	if visible != int64(len(normalizedIDs)) {
		return gorm.ErrRecordNotFound
	}

	res := tx.Table(s.TableName).Where("{{.Pk}} IN ?", normalizedIDs).Delete(&{{.ClassName}}{})
	if err := res.Error; err != nil {
		return err
	}
	if res.RowsAffected != int64(len(normalizedIDs)) {
		return gorm.ErrRecordNotFound
	}
	return nil
	})
}

func normalize{{.ClassName}}IDs(ids interface{}) ([]int32, error) {
	raw, ok := ids.([]int32)
	if !ok {
		return nil, fmt.Errorf("invalid {{.Pk}} ids type %T", ids)
	}
	seen := make(map[int32]struct{}, len(raw))
	result := make([]int32, 0, len(raw))
	for _, id := range raw {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}
`

const StructTmpl = `import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	{{range .ImportPkgPaths}}{{.}} ` + "\n" + `{{end}}
)

// {{.ModelStructName}} {{.StructComment}}
type {{.ModelStructName}} struct {
    {{range .Fields}}
    {{if .MultilineComment -}}
	/*
{{.ColumnComment}}
    */
	{{end -}}
    {{.Name}} {{.Type}} ` + "`{{.Tags}}` " +
	"{{if not .MultilineComment}}{{if .ColumnComment}}// {{.ColumnComment}}{{end}}{{end}}" +
	`{{end}}
}
`
