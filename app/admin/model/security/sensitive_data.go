package security

import (
	"encoding/json"
	"fmt"
	adminmodel "go-build-admin/app/admin/model"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

// SensitiveDatum 敏感数据规则表
type SecuritySensitiveData struct {
	ID           int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`
	Name         string `gorm:"column:name;not null;comment:规则名称" json:"name"`                       // 规则名称
	Controller   string `gorm:"column:controller;not null;comment:控制器" json:"controller"`            // 控制器
	ControllerAs string `gorm:"column:controller_as;not null;comment:控制器别名" json:"controller_as"`    // 控制器别名
	DataTable    string `gorm:"column:data_table;not null;comment:对应数据表" json:"data_table"`          // 对应数据表
	PrimaryKey   string `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`        // 数据表主键
	DataFields   string `gorm:"column:data_fields;comment:敏感数据字段" json:"data_fields"`                // 敏感数据字段
	Status       string `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	Connection   string `gorm:"column:connection;not null;default:'';comment:数据库连接配置标识" json:"connection"`
	UpdateTime   int64  `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"` // 更新时间
	CreateTime   int64  `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
}

type OutSensitiveData struct {
	SecuritySensitiveData
	DataFields []string `json:"data_fields"`
}

type SensitiveDataModel struct {
	adminmodel.BaseModel
	config *conf.Configuration
}

func NewSensitiveDataModel(sqlDB *gorm.DB, config *conf.Configuration) *SensitiveDataModel {
	return &SensitiveDataModel{
		BaseModel: adminmodel.NewBaseModel(config.Database.Prefix+"security_sensitive_data", "id", "controller", sqlDB),
		config:    config,
	}
}

func (s *SensitiveDataModel) DealData(ctx *gin.Context, data *SecuritySensitiveData) (*OutSensitiveData, error) {
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

func (s *SensitiveDataModel) GetOne(ctx *gin.Context, id int32) (sensitiveData SecuritySensitiveData, err error) {
	err = s.DBFor(ctx).Model(&SecuritySensitiveData{}).Where("id=?", id).First(&sensitiveData).Error
	return
}

func (s *SensitiveDataModel) List(ctx *gin.Context) ([]*OutSensitiveData, int64, error) {
	whereS, whereP, orderS, limit, offset, err := adminmodel.QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	var total int64 = 0
	list := []*SecuritySensitiveData{}

	db := s.DBFor(ctx).Model(&SecuritySensitiveData{}).Where(whereS, whereP...)
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

func (s *SensitiveDataModel) Add(ctx *gin.Context, data SecuritySensitiveData) error {
	if data.PrimaryKey == "" {
		data.PrimaryKey = "id"
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(data.DataFields), &fields); err != nil {
		return err
	}
	fieldNames := make([]string, 0, len(fields))
	for field := range fields {
		fieldNames = append(fieldNames, field)
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		_, err := resolveRulePolicy(tx, s.config.Database.Prefix, data.DataTable, "sensitive", data.PrimaryKey, fieldNames)
		if err != nil {
			return err
		}
		if data.Status == "1" {
			if err := rejectDuplicateEnabledControllerAs(tx, s.config.Database.Prefix+"security_sensitive_data", data.ControllerAs, 0); err != nil {
				return err
			}
		}
		return tx.Create(&data).Error
	})
}

func (s *SensitiveDataModel) Edit(ctx *gin.Context, data SecuritySensitiveData) error {
	if data.PrimaryKey == "" {
		data.PrimaryKey = "id"
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(data.DataFields), &fields); err != nil {
		return err
	}
	fieldNames := make([]string, 0, len(fields))
	for field := range fields {
		fieldNames = append(fieldNames, field)
	}
	updates := map[string]any{
		"name": data.Name, "controller": data.Controller, "controller_as": data.ControllerAs,
		"data_table": data.DataTable, "primary_key": data.PrimaryKey, "data_fields": data.DataFields,
		"status": data.Status, "connection": data.Connection,
	}
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		_, err := resolveRulePolicy(tx, s.config.Database.Prefix, data.DataTable, "sensitive", data.PrimaryKey, fieldNames)
		if err != nil {
			return err
		}
		if data.Status == "1" {
			if err := rejectDuplicateEnabledControllerAs(tx, s.config.Database.Prefix+"security_sensitive_data", data.ControllerAs, data.ID); err != nil {
				return err
			}
		}
		result = tx.Model(&SecuritySensitiveData{}).Where("id = ?", data.ID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&SecuritySensitiveData{}).Where("id = ?", data.ID).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return gorm.ErrRecordNotFound
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *SensitiveDataModel) UpdateStatus(ctx *gin.Context, id int32, status string) error {
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		if status == "1" {
			var current SecuritySensitiveData
			if err := tx.Where("id = ?", id).First(&current).Error; err != nil {
				return err
			}
			if err := rejectDuplicateEnabledControllerAs(tx, s.config.Database.Prefix+"security_sensitive_data", current.ControllerAs, id); err != nil {
				return err
			}
		}
		result = tx.Model(&SecuritySensitiveData{}).Where("id = ?", id).Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&SecuritySensitiveData{}).Where("id = ?", id).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return gorm.ErrRecordNotFound
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *SensitiveDataModel) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid security sensitive data ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid security sensitive data id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var list []SecuritySensitiveData
		query := tx.Model(&SecuritySensitiveData{})
		if err := query.Where("id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		del := query.Where("id IN ?", normalized).Delete(nil)
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
