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

type Table struct {
	ColumnFields         []string `json:"columnFields"`
	Comment              string   `json:"comment"`
	ControllerFile       string   `json:"controllerFile"`
	DefaultSortField     string   `json:"defaultSortField"`
	DefaultSortType      string   `json:"defaultSortType"`
	DesignChange         []string `json:"designChange"`
	Empty                bool     `json:"empty"`
	FormFields           []string `json:"formFields"`
	GenerateRelativePath string   `json:"generateRelativePath"`
	IsCommonModel        int32    `json:"isCommonModel"`
	Name                 string   `json:"name"`
	QuickSearchField     []string `json:"quickSearchField"`
	Rebuild              string   `json:"rebuild"`
	ValidateFile         string   `json:"validateFile"`
	ModelFile            string   `json:"modelFile"`
	WebViewsDir          string   `json:"webViewsDir"`
}

type TableAttr struct {
	Operator string `json:"operator"`
	Sortable string `json:"sortable"`
	Width    string `json:"width"`
	Render   string `json:"render"`
}

type FormAttr struct {
	Validator    []string `json:"validator"`
	ValidatorMsg string   `json:"validatorMsg"`
	Step         string   `json:"step"`
}

type Field struct {
	AutoIncrement string    `json:"autoIncrement"`
	Comment       string    `json:"comment"`
	DataType      string    `json:"dataType"`
	Default       string    `json:"default"`
	DesignType    string    `json:"designType"`
	Form          FormAttr  `json:"form"`
	Name          string    `json:"name"`
	Null          string    `json:"null"`
	PrimaryKey    string    `json:"primaryKey"`
	Table         TableAttr `json:"table"`
	Type          string    `json:"type"`
	Unsigned      string    `json:"unsigned"`
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
