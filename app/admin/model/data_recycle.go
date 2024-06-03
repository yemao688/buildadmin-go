package model

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const TableNameSecurityDataRecycle = "ba_security_data_recycle"

// DataRecycle 回收规则表
type DataRecycle struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Name         string `gorm:"column:name;not null;comment:规则名称" json:"name"`                       // 规则名称
	Controller   string `gorm:"column:controller;not null;comment:控制器" json:"controller"`            // 控制器
	ControllerAs string `gorm:"column:controller_as;not null;comment:控制器别名" json:"controller_as"`    // 控制器别名
	DataTable    string `gorm:"column:data_table;not null;comment:对应数据表" json:"data_table"`          // 对应数据表
	PrimaryKey   string `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`        // 数据表主键
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	UpdateTime   int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"`   // 更新时间
	CreateTime   int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`   // 创建时间
}

func (*DataRecycle) TableName() string {
	return TableNameSecurityDataRecycle
}

type DataRecycleModel struct {
	BaseModel
}

func NewDataRecycleModel(sqlDB *gorm.DB) *DataRecycleModel {
	return &DataRecycleModel{
		BaseModel: BaseModel{
			TableName:        TableNameSecurityDataRecycle,
			Key:              "id",
			QuickSearchField: "name",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *DataRecycleModel) GetOne(ctx *gin.Context, id int32) (data DataRecycle, err error) {
	err = s.sqlDB.Table(s.TableName).Where("id=?", id).First(&data).Error
	return
}

func (s *DataRecycleModel) List(ctx *gin.Context) (list []DataRecycle, total int64, err error) {
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

func (s *DataRecycleModel) Add(ctx *gin.Context, data DataRecycle) error {
	err := s.sqlDB.Table(s.TableName).Create(&data).Error
	return err
}

func (s *DataRecycleModel) Edit(ctx *gin.Context, data DataRecycle) error {
	err := s.sqlDB.Table(s.TableName).Updates(&data).Error
	return err
}

func (s *DataRecycleModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
