package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"buildadmin-go/internal/pkg/data_scope"

	"gorm.io/gorm/schema"
)

// TableName 经全局命名策略解析（前缀安全），对齐真实表 crud_log。
func (Log) TableName(namer schema.Namer) string {
	return namer.TableName("crud_log")
}

// Log CRUD记录表
type Log struct {
	ID         int32       `gorm:"column:id;type:int(11) unsigned;not null;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                                          // ID
	AdminID    int32       `gorm:"column:admin_id;type:int(11) unsigned;not null;default:0;index:idx_admin_id;comment:管理员ID" json:"admin_id"`                                            // ID
	Tablename  string      `gorm:"column:table_name;type:varchar(200) default '';not null;comment:数据表名" json:"table_name"`                                                               // 数据表名
	Table      JSON_TABLE  `gorm:"column:table;type:text;comment:数据表数据" json:"table"`                                                                                                    // 数据表数据
	Fields     JSON_FIELDS `gorm:"column:fields;type:text;comment:字段数据" json:"fields"`                                                                                                   // 字段数据
	Status     string      `gorm:"column:status;type:enum('delete','success','error','start');not null;default:start;comment:状态:delete=已删除,success=成功,error=失败,start=生成中" json:"status"` // 状态:delete=已删除,success=成功,error=失败,start=生成中
	Comment    string      `gorm:"column:comment;type:varchar(255) default '';not null;comment:表注释" json:"comment"`
	Connection string      `gorm:"column:connection;type:varchar(100) default '';not null;comment:数据库连接配置标识" json:"connection"`
	Sync       int         `gorm:"column:sync;type:int(11);not null;default:0;comment:同步记录" json:"sync"`
	CreateTime int64       `gorm:"autoCreateTime;column:create_time;type:bigint(16) unsigned default null;comment:创建时间" json:"create_time"` // 创建时间
}

type JSON_TABLE Table

func (j *JSON_TABLE) Scan(value interface{}) error {
	bytes, err := scanBytes(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, j)
}

func (j JSON_TABLE) Value() (driver.Value, error) {
	bytes, err := json.Marshal(j)
	return string(bytes), err
}

type JSON_FIELDS []Field

func (j *JSON_FIELDS) Scan(value interface{}) error {
	bytes, err := scanBytes(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, j)
}

// scanBytes 兼容 TEXT 列在不同驱动下的返回类型（MySQL 多为 []byte，sqlite 为 string）。
func scanBytes(value interface{}) ([]byte, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	case nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("cannot scan %T as JSON bytes", value)
	}
}

func (j JSON_FIELDS) Value() (driver.Value, error) {
	bytes, err := json.Marshal(j)
	return string(bytes), err
}

type ChangeField struct {
	Type    string `json:"type"`    //变更类型
	Index   int32  `json:"index"`   //索引
	OldName string `json:"oldName"` //旧名称
	NewName string `json:"newName"` //新名称
	Sync    bool   `json:"sync"`    //是否同步到数据表
	After   string `json:"after"`   //在什么后
	Risk    string `json:"risk,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// IndexSpec 声明业务表上的一条数据库索引（目前支持唯一索引；普通索引
// unique=false 同样可声明）。apply 物化表结构时同步建/比对这些索引。
type IndexSpec struct {
	Name    string   `json:"name"`    //索引名（必填，数据库内唯一）
	Unique  bool     `json:"unique"`  //是否唯一索引
	Columns []string `json:"columns"` //索引列（至少一列，引用 spec 中真实字段）
}

type Table struct {
	DataScope            *data_scope.Config `json:"dataScope,omitempty"`      //数据权限配置
	Name                 string             `json:"name"`                     //数据表名
	Comment              string             `json:"comment"`                  //数据表注释
	DatabaseConnection   string             `json:"databaseConnection"`       //数据库连接配置标识
	QuickSearchField     []string           `json:"quickSearchField"`         //表格快速搜索字段
	DefaultSortField     string             `json:"defaultSortField"`         //表格默认排序字段
	FormFields           []string           `json:"formFields"`               //作为表单项的字段
	ColumnFields         []string           `json:"columnFields"`             //作为表格列的字段
	DefaultSortType      string             `json:"defaultSortType"`          //排序方式
	GenerateRelativePath string             `json:"generateRelativePath"`     //生成代码的相对位置
	IsCommonModel        int                `json:"isCommonModel"`            //是否公共模型
	ModelFile            string             `json:"modelFile"`                //生成的数据模型位置
	ControllerFile       string             `json:"controllerFile"`           //生成的控制器位置
	ValidateFile         string             `json:"validateFile"`             //生成的验证器位置
	WebViewsDir          string             `json:"webViewsDir"`              //WEB端视图目录
	DesignChange         []ChangeField      `json:"designChange"`             //表设计变更
	Rebuild              string             `json:"rebuild"`                  //是否重建
	Empty                bool               `json:"empty"`                    //表格是否有数据,后台增加
	RegisterOnly         bool               `json:"registerOnly,omitempty"`    //仅登记不生成：受保护核心表（user 等）以 crud_log 登记供级联校验/对账，不生成任何产物
	Indexes              []IndexSpec        `json:"indexes,omitempty"`         //表级索引声明（apply 物化）
	GeneratedFiles       []string           `json:"generatedFiles,omitempty"` //最近一次成功生成的文件清单
	Manifest             *CRUDFileManifest  `json:"manifest,omitempty" gorm:"-"`
}

type CRUDFileManifest struct {
	Generated []string `json:"generated"`
	Shared    []string `json:"shared"`
}

type TableAttr struct {
	Width      int    `json:"width"`      //表格列宽度
	Operator   string `json:"operator"`   //公共搜索操作符
	Sortable   string `json:"sortable"`   //字段排序
	Render     string `json:"render"`     //渲染方案
	TimeFormat string `json:"timeFormat"` //格式化方式

	Label              string              `json:"label"`                                                //关联表格列属性
	Show               string              `json:"show"`                                                 //关联表格列属性
	ComSearchRender    string              `json:"comSearchRender"`                                      //关联表格列属性
	ComSearchInputAttr ComSearchInputAttrs `json:"comSearchInputAttr" mapstructure:"comSearchInputAttr"` //公共搜索输入属性
	Remote             string              `json:"remote"`                                               //关联表格列属性
}

// ComSearchInputAttrs is the normalized form of the designer's textarea
// attribute syntax. JSON clients may send either that string or an object.
type ComSearchInputAttrs map[string]any

func (attrs *ComSearchInputAttrs) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		parsed, err := ParseComSearchInputAttrs(text)
		if err != nil {
			return err
		}
		*attrs = parsed
		return nil
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("comSearchInputAttr must be a string or object: %w", err)
	}
	*attrs = ComSearchInputAttrs(object)
	return nil
}

// ParseComSearchInputAttrs parses one designer attribute per line. Empty
// lines are ignored, values retain text after the first '=', and booleans and
// numbers retain their useful scalar types.
func ParseComSearchInputAttrs(input string) (ComSearchInputAttrs, error) {
	result := ComSearchInputAttrs{}
	input = strings.TrimSpace(strings.ReplaceAll(input, "\r\n", "\n"))
	if input == "" {
		return result, nil
	}
	for lineNumber, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("comSearchInputAttr entry %d is malformed: %q", lineNumber+1, line)
		}
		key := strings.TrimSpace(parts[0])
		keyParts := strings.Split(key, ".")
		for _, part := range keyParts {
			if part == "" {
				return nil, fmt.Errorf("comSearchInputAttr entry %d has an empty key segment: %q", lineNumber+1, key)
			}
		}
		value := parseComSearchInputAttrValue(parts[1])
		if len(keyParts) == 1 {
			result[key] = value
			continue
		}
		child, ok := result[keyParts[0]].(map[string]any)
		if !ok {
			child = map[string]any{}
			result[keyParts[0]] = child
		}
		child[strings.Join(keyParts[1:], ".")] = value
	}
	return result, nil
}

func parseComSearchInputAttrValue(value string) any {
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		return number
	}
	return value
}

// BoolOrString 兼容上游前端对多选开关可能传入布尔值或字符串的情况:
// true 归一化为 "1",false 归一化为 "",字符串按原样保留。
type BoolOrString string

func (b *BoolOrString) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*b = BoolOrString(s)
		return nil
	}
	var v bool
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("select-multi 类字段只接受布尔或字符串: %w", err)
	}
	if v {
		*b = "1"
	} else {
		*b = ""
	}
	return nil
}

type FormAttr struct {
	Validator    []string `json:"validator"`    //验证规则
	ValidatorMsg string   `json:"validatorMsg"` //验证错误提示

	Rows        int          `json:"rows"`         //富文本行数
	SelectMulti BoolOrString `json:"select-multi"` //下拉框多选
	ImageMulti  BoolOrString `json:"image-multi"`  //图片多选上传
	FileMulti   BoolOrString `json:"file-multi"`   //文件多选上传
	Step        float64      `json:"step"`         //步进值

	RemotePk                string `json:"remote-pk" mapstructure:"remotePk"`                                 //远程下拉value字段
	RemoteField             string `json:"remote-field" mapstructure:"remoteField"`                           //远程下拉label字段
	RemoteTable             string `json:"remote-table" mapstructure:"remoteTable"`                           //关联数据表
	RemoteController        string `json:"remote-controller" mapstructure:"remoteController"`                 //关联表的控制器
	RemoteModel             string `json:"remote-model" mapstructure:"remoteModel"`                           //关联表的模型
	RemoteUrl               string `json:"remote-url" mapstructure:"remoteUrl"`                               //远程下拉URL
	RemotePrimaryTableAlias string `json:"remote-primary-table-alias" mapstructure:"remotePrimaryTableAlias"` //远程主表别名
	RemoteSourceConfigType  string `json:"remote-source-config-type" mapstructure:"remoteSourceConfigType"`   //远程下拉来源类型(crud/custom)
	RelationFields          string `json:"relation-fields" mapstructure:"relationFields"`                     //关联表显示字段
}

type Field struct {
	Title             string    `json:"title"`             //生成为
	Name              string    `json:"name"`              //字段名
	Type              string    `json:"type"`              //字段类型
	DataType          string    `json:"dataType"`          //enum,set数据值
	Length            int       `json:"length"`            //长度
	Precision         int       `json:"precision"`         //小数点
	Default           string    `json:"default"`           //字段默认值
	DefaultType       string    `json:"defaultType"`       //默认值类型:NONE/NULL/EMPTY STRING/INPUT
	Null              bool      `json:"null"`              //允许NULL
	PrimaryKey        bool      `json:"primaryKey"`        //主键
	Unsigned          bool      `json:"unsigned"`          //无符号
	AutoIncrement     bool      `json:"autoIncrement"`     //自动递增
	Comment           string    `json:"comment"`           //字段注释
	DesignType        string    `json:"designType"`        //字段类型
	FormBuildExclude  bool      `json:"formBuildExclude"`  //表单表格字段预定义
	TableBuildExclude bool      `json:"tableBuildExclude"` //表单表格字段预定义
	Table             TableAttr `json:"table"`             //字段表格属性
	Form              FormAttr  `json:"form"`              //字段表单属性

	OriginalDesignType string `json:"originalDesignType"`
}

