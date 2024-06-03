package model

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

const TableNameSecuritySensitiveData = "ba_security_sensitive_data"

// SensitiveDatum 敏感数据规则表
type SensitiveData struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Name         string `gorm:"column:name;not null;comment:规则名称" json:"name"`                       // 规则名称
	Controller   string `gorm:"column:controller;not null;comment:控制器" json:"controller"`            // 控制器
	ControllerAs string `gorm:"column:controller_as;not null;comment:控制器别名" json:"controller_as"`    // 控制器别名
	DataTable    string `gorm:"column:data_table;not null;comment:对应数据表" json:"data_table"`          // 对应数据表
	PrimaryKey   string `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`        // 数据表主键
	DataFields   string `gorm:"column:data_fields;comment:敏感数据字段" json:"data_fields"`                // 敏感数据字段
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	UpdateTime   int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"`   // 更新时间
	CreateTime   int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`   // 创建时间
}

func (*SensitiveData) TableName() string {
	return TableNameSecuritySensitiveData
}

type OutSensitiveData struct {
	SensitiveData
	DataFields []string `json:"data_fields"`
}

type SensitiveDataModel struct {
	BaseModel
}

func NewSensitiveDataModel(sqlDB *gorm.DB) *SensitiveDataModel {
	return &SensitiveDataModel{
		BaseModel: BaseModel{
			TableName:        TableNameSecuritySensitiveData,
			Key:              "id",
			QuickSearchField: "controller",
			DataLimit:        "",
			sqlDB:            sqlDB,
		},
	}
}

func (s *SensitiveDataModel) DealData(ctx *gin.Context, data *SensitiveData) (*OutSensitiveData, error) {
	outSensitiveData := OutSensitiveData{}
	if err := copier.Copy(&outSensitiveData, data); err != nil {
		return nil, err
	}
	fieldData := []string{}
	result := map[string]string{}
	if err := json.Unmarshal([]byte(data.DataFields), &result); err != nil {
		return nil, err
	}
	for _, v := range result {
		fieldData = append(fieldData, v)
	}
	outSensitiveData.DataFields = fieldData
	return &outSensitiveData, nil
}

func (s *SensitiveDataModel) GetOne(ctx *gin.Context, id int32) (sensitiveData SensitiveData, err error) {
	err = s.sqlDB.Table(s.TableName).Where("id=?", id).First(&sensitiveData).Error
	return
}

func (s *SensitiveDataModel) List(ctx *gin.Context) ([]*OutSensitiveData, int64, error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	var total int64 = 0
	list := []*SensitiveData{}

	db := s.sqlDB.Table(s.TableName).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	result := []*OutSensitiveData{}
	for _, v := range list {
		outSensitiveData, err := s.DealData(ctx, v)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, outSensitiveData)
	}
	return result, total, err
}

func (s *SensitiveDataModel) Add(ctx *gin.Context, data SensitiveData) error {
	err := s.sqlDB.Table(s.TableName).Create(&data).Error
	return err
}

func (s *SensitiveDataModel) Edit(ctx *gin.Context, data SensitiveData) error {
	err := s.sqlDB.Table(s.TableName).Updates(&data).Error
	return err
}

func (s *SensitiveDataModel) Del(ctx *gin.Context, ids interface{}) error {
	err := s.sqlDB.Table(s.TableName).Scopes(LimitAdminIds(ctx)).Where(" id in ? ", ids).Delete(nil).Error
	return err
}
