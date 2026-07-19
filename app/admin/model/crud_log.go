package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CrudLog CRUD记录表
type CrudLog struct {
	ID         int32       `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	AdminID    int32       `gorm:"column:admin_id;not null;comment:管理员ID" json:"admin_id"`                                                 // ID
	Tablename  string      `gorm:"column:table_name;not null;comment:数据表名" json:"table_name"`                                              // 数据表名
	Table      JSON_TABLE  `gorm:"column:table;comment:数据表数据" json:"table"`                                                                // 数据表数据
	Fields     JSON_FIELDS `gorm:"column:fields;comment:字段数据" json:"fields"`                                                               // 字段数据
	Status     string      `gorm:"column:status;not null;default:start;comment:状态:delete=已删除,success=成功,error=失败,start=生成中" json:"status"` // 状态:delete=已删除,success=成功,error=失败,start=生成中
	Comment    string      `gorm:"column:comment;comment:表注释" json:"comment"`
	Connection string      `gorm:"column:connection;not null;default:'';comment:数据库连接配置标识" json:"connection"`
	Sync       int         `gorm:"column:sync;default:0;comment:同步记录" json:"sync"`
	CreateTime int64       `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
}

type CrudLogModel struct {
	BaseModel
	enforcer data_scope.Enforcer
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
	After   string `json:"after"`   //在什么后
}

type Table struct {
	DataScope            *data_scope.Config `json:"dataScope,omitempty"`      //数据权限配置
	Name                 string             `json:"name"`                     //数据表名
	Comment              string             `json:"comment"`                  //数据表注释
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

	Label           string `json:"label"`           //关联表格列属性
	Show            string `json:"show"`            //关联表格列属性
	ComSearchRender string `json:"comSearchRender"` //关联表格列属性
	Remote          string `json:"remote"`          //关联表格列属性
}

type FormAttr struct {
	Validator    []string `json:"validator"`    //验证规则
	ValidatorMsg string   `json:"validatorMsg"` //验证错误提示

	Rows        int    `json:"rows"`         //富文本行数
	SelectMulti string `json:"select-multi"` //下拉框多选
	ImageMulti  string `json:"image-multi"`  //图片多选上传
	FileMulti   string `json:"file-multi"`   //文件多选上传
	Step        int    `json:"step"`         //步进值

	RemotePk         string `json:"remote-pk" mapstructure:"remotePk"`                 //远程下拉value字段
	RemoteField      string `json:"remote-field" mapstructure:"remoteField"`           //远程下拉label字段
	RemoteTable      string `json:"remote-table" mapstructure:"remoteTable"`           //关联数据表
	RemoteController string `json:"remote-controller" mapstructure:"remoteController"` //关联表的控制器
	RemoteModel      string `json:"remote-model" mapstructure:"remoteModel"`           //关联表的模型
	RemoteUrl        string `json:"remote-url" mapstructure:"remoteUrl"`               //远程下拉URL
	RelationFields   string `json:"relation-fields" mapstructure:"relationFields"`     //关联表显示字段
}

type Field struct {
	Title             string    `json:"title"`             //生成为
	Name              string    `json:"name"`              //字段名
	Type              string    `json:"type"`              //字段类型
	DataType          string    `json:"dataType"`          //enum,set数据值
	Length            int       `json:"length"`            //长度
	Precision         int       `json:"precision"`         //小数点
	Default           string    `json:"default"`           //字段默认值
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

func NewCrudLogModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *CrudLogModel {
	return &CrudLogModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "crud_log",
			Key:              "id",
			QuickSearchField: "table_name",
			sqlDB:            sqlDB,
		},
		enforcer: enforcer,
	}
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// crud_log.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *CrudLogModel) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

func (s *CrudLogModel) GetByTableName(ctx *gin.Context, table string) (crudLog CrudLog, err error) {
	err = s.sqlDB.Model(&CrudLog{}).Scopes(s.scoped(ctx)).Where("table_name=?", table).Order("create_time desc").Take(&crudLog).Error
	return
}

// HasAnyByTableName intentionally bypasses data scope. It is used only to
// distinguish a first generation from regeneration; exposing the log row is
// not required. A scoped lookup would let another administrator overwrite a
// handwritten file merely because somebody else generated that table.
func (s *CrudLogModel) HasAnyByTableName(table string) (bool, error) {
	prefix := strings.TrimSuffix(s.TableName, "crud_log")
	names := []string{table, strings.TrimPrefix(table, prefix)}
	if prefix != "" {
		names = append(names, prefix+strings.TrimPrefix(table, prefix))
	}
	var count int64
	err := s.sqlDB.Model(&CrudLog{}).Where("table_name IN ?", names).Count(&count).Error
	return count > 0, err
}

func (s *CrudLogModel) GetOne(ctx *gin.Context, id int32) (crudLog CrudLog, err error) {
	err = s.sqlDB.Model(&CrudLog{}).Scopes(s.scoped(ctx)).Where("id=?", id).First(&crudLog).Error
	return
}

func (s *CrudLogModel) List(ctx *gin.Context) (list []CrudLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.sqlDB.Model(&CrudLog{}).Scopes(s.scoped(ctx)).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Select("id", "table_name", "status", "create_time").Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *CrudLogModel) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid crud log ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid crud log id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.sqlDB.Transaction(func(tx *gorm.DB) error {
		var list []CrudLog
		scoped := tx.Model(&CrudLog{}).Scopes(s.scoped(ctx))
		if err := scoped.Where("id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		del := scoped.Where("id IN ?", normalized).Delete(nil)
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// 记录CRUD状态
func (s *CrudLogModel) RecordCrudStatus(ctx *gin.Context, data CrudLog) (int32, error) {
	if s.enforcer == nil {
		return 0, data_scope.ErrScopedAccessDenied
	}
	actor, err := s.enforcer.Actor(ctx)
	if err != nil {
		return 0, err
	}
	data.AdminID = actor.AdminID

	if data.ID != 0 {
		result := s.sqlDB.Model(&CrudLog{}).Scopes(s.scoped(ctx)).Where("id=?", data.ID).Update("status", data.Status)
		if result.Error != nil {
			return 0, result.Error
		}
		if result.RowsAffected != 1 {
			return 0, gorm.ErrRecordNotFound
		}
		return data.ID, nil
	}
	if err := s.sqlDB.Create(&data).Error; err != nil {
		return 0, err
	}
	return data.ID, nil
}

// RecordCrudError marks a generation as failed and preserves the failing
// stage/message for operators. It intentionally uses the same scoped update
// path as normal status transitions.
func (s *CrudLogModel) RecordCrudError(ctx *gin.Context, id int32, message string) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	result := s.sqlDB.Model(&CrudLog{}).Scopes(s.scoped(ctx)).Where("id=?", id).Updates(map[string]interface{}{
		"status":  "error",
		"comment": message,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
