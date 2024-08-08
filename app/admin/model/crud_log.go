package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameCrudLog = "ba_crud_log"

// CrudLog CRUD记录表
type CrudLog struct {
	ID         int32       `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                           // ID
	Tablename  string      `gorm:"column:table_name;not null;comment:数据表名" json:"table_name"`                                              // 数据表名
	Table      JSON_TABLE  `gorm:"column:table;comment:数据表数据" json:"table"`                                                                // 数据表数据
	Fields     JSON_FIELDS `gorm:"column:fields;comment:字段数据" json:"fields"`                                                               // 字段数据
	Status     string      `gorm:"column:status;not null;default:start;comment:状态:delete=已删除,success=成功,error=失败,start=生成中" json:"status"` // 状态:delete=已删除,success=成功,error=失败,start=生成中
	CreateTime int64       `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`                                      // 创建时间
}

type CrudLogModel struct {
	BaseModel
}

type JSON_TABLE Table

func (j *JSON_TABLE) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
}

func (j JSON_TABLE) Value() (driver.Value, error) {
	bytes, err := json.Marshal(j)
	return string(bytes), err
}

type JSON_FIELDS []Field

func (j *JSON_FIELDS) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, j)
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
}

type Table struct {
	Name                 string        `json:"name"`                 //数据表名
	Comment              string        `json:"comment"`              //数据表注释
	QuickSearchField     []string      `json:"quickSearchField"`     //表格快速搜索字段
	DefaultSortField     string        `json:"defaultSortField"`     //表格默认排序字段
	FormFields           []string      `json:"formFields"`           //作为表单项的字段
	ColumnFields         []string      `json:"columnFields"`         //作为表格列的字段
	DefaultSortType      string        `json:"defaultSortType"`      //排序方式
	GenerateRelativePath string        `json:"generateRelativePath"` //生成代码的相对位置
	IsCommonModel        int           `json:"isCommonModel"`        //是否公共模型
	ModelFile            string        `json:"modelFile"`            //生成的数据模型位置
	ControllerFile       string        `json:"controllerFile"`       //生成的控制器位置
	ValidateFile         string        `json:"validateFile"`         //生成的验证器位置
	WebViewsDir          string        `json:"webViewsDir"`          //WEB端视图目录
	DesignChange         []ChangeField `json:"designChange"`         //表设计变更
	Rebuild              string        `json:"rebuild"`              //是否重建
	Empty                bool          `json:"empty"`                //表格是否有数据
}

type TableAttr struct {
	Width      string `json:"width"`      //表格列宽度
	Operator   string `json:"operator"`   //公共搜索操作符
	Sortable   string `json:"sortable"`   //字段排序
	Render     string `json:"render"`     //渲染方案
	TimeFormat string `json:"timeFormat"` //格式化方式
}

type FormAttr struct {
	Validator    []string `json:"validator"`    //验证规则
	ValidatorMsg string   `json:"validatorMsg"` //验证错误提示
	Rows         string   `json:"rows"`         //富文本行数

	SelectMulti string `json:"select-multi"` //下拉框多选

	RemotePk         string `json:"remote-pk"`         //远程下拉value字段
	RemoteField      string `json:"remote-field"`      //远程下拉label字段
	RemoteTable      string `json:"remote-table"`      //关联数据表
	RemoteController string `json:"remote-controller"` //关联表的控制器
	RemoteModel      string `json:"remote-model"`      //关联表的模型
	RemoteUrl        string `json:"remote-url"`        //远程下拉URL
	RelationFields   string `json:"relation-fields"`   //关联表显示字段

	ImageMulti string `json:"image-multi"` //图片多选上传
	FileMulti  string `json:"file-multi"`  //文件多选上传
	Step       string `json:"step"`        //步进值
}

type Field struct {
	Title             string    `json:"title"`             //生成为
	Name              string    `json:"name"`              //字段名
	DataType          string    `json:"dataType"`          //enum,set数据值
	Comment           string    `json:"comment"`           //字段注释
	DesignType        string    `json:"designType"`        //字段类型
	TableBuildExclude string    `json:"tableBuildExclude"` //表单表格字段预定义
	FormBuildExclude  string    `json:"formBuildExclude"`  //表单表格字段预定义
	Table             TableAttr `json:"table"`             //字段表格属性
	Form              FormAttr  `json:"form"`              //字段表单属性
	Type              string    `json:"type"`              //字段类型
	Length            string    `json:"length"`            //长度
	Precision         string    `json:"precision"`         //小数点
	Default           string    `json:"default"`           //字段默认值
	Null              string    `json:"null"`              //允许NULL
	PrimaryKey        string    `json:"primaryKey"`        //主键
	Unsigned          string    `json:"unsigned"`          //无符号
	AutoIncrement     string    `json:"autoIncrement"`     //自动递增

	OriginalDesignType string `json:"originalDesignType"`
}

func NewCrudLogModel(sqlDB *gorm.DB) *CrudLogModel {
	return &CrudLogModel{
		BaseModel: BaseModel{
			TableName:        TableNameCrudLog,
			Key:              "id",
			QuickSearchField: "table_name",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *CrudLogModel) GetByTableName(ctx *gin.Context, table string) (crudLog CrudLog, err error) {
	err = s.sqlDB.Table(s.TableName).Where("table_name=?", table).Order("create_time desc").Take(&crudLog).Error
	return
}

func (s *CrudLogModel) GetOne(ctx *gin.Context, id int32) (crudLog CrudLog, err error) {
	err = s.sqlDB.Table(s.TableName).Where("id=?", id).First(&crudLog).Error
	return
}

func (s *CrudLogModel) List(ctx *gin.Context) (list []CrudLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.sqlDB.Table(s.TableName).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *CrudLogModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}

// 记录CRUD状态
func (s *CrudLogModel) RecordCrudStatus(data CrudLog) int32 {
	if data.ID != 0 {
		s.sqlDB.Table(s.TableName).Where("id=?", data.ID).Update("status", data.Status)
		return data.ID
	}
	s.sqlDB.Table(s.TableName).Create(&data)
	return data.ID
}
