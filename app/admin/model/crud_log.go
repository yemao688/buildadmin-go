package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameCrudLog = "ba_crud_log"

// CrudLog CRUD记录表
type CrudLog struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                           // ID
	Tablename  string `gorm:"column:table_name;not null;comment:数据表名" json:"table_name"`                                              // 数据表名
	Table      string `gorm:"column:table;comment:数据表数据" json:"table"`                                                                // 数据表数据
	Fields     string `gorm:"column:fields;comment:字段数据" json:"fields"`                                                               // 字段数据
	Status     string `gorm:"column:status;not null;default:start;comment:状态:delete=已删除,success=成功,error=失败,start=生成中" json:"status"` // 状态:delete=已删除,success=成功,error=失败,start=生成中
	CreateTime int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`                                                     // 创建时间
}

type CrudLogModel struct {
	BaseModel
	sqlDB *gorm.DB
}

func NewCrudLogModel(sqlDB *gorm.DB) *CrudLogModel {
	return &CrudLogModel{
		BaseModel: BaseModel{
			TableName:        TableNameCrudLog,
			Key:              "id",
			QuickSearchField: "table_name",
			DataLimit:        "",
		},
		sqlDB: sqlDB}
}

func (s *CrudLogModel) List(ctx *gin.Context) (list []CrudLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	err = s.sqlDB.Table(s.TableName).Scopes(Total(whereS, whereP, &total)).Where(whereS, whereP...).Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}
