package security

import (
	"encoding/json"
	"fmt"
	adminmodel "go-build-admin/internal/admin/repository"
	"go-build-admin/internal/conf"
	"go-build-admin/internal/model"
	persistence "go-build-admin/internal/pkg/persistence"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

type OutSensitiveData struct {
	model.SecuritySensitiveData
	DataFields []string `json:"data_fields"`
}

type SensitiveDataRepository struct {
	persistence.BaseModel
	config *conf.Configuration
}

func NewSensitiveDataRepository(sqlDB *gorm.DB, config *conf.Configuration) *SensitiveDataRepository {
	return &SensitiveDataRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"security_sensitive_data", "id", "controller", sqlDB),
		config:    config,
	}
}

func (s *SensitiveDataRepository) DealData(ctx *gin.Context, data *model.SecuritySensitiveData) (*OutSensitiveData, error) {
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

func (s *SensitiveDataRepository) GetOne(ctx *gin.Context, id int32) (sensitiveData model.SecuritySensitiveData, err error) {
	err = s.DBFor(ctx).Model(&model.SecuritySensitiveData{}).Where("id=?", id).First(&sensitiveData).Error
	return
}

func (s *SensitiveDataRepository) List(ctx *gin.Context) ([]*OutSensitiveData, int64, error) {
	whereS, whereP, orderS, limit, offset, err := adminmodel.QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	var total int64 = 0
	list := []*model.SecuritySensitiveData{}

	db := s.DBFor(ctx).Model(&model.SecuritySensitiveData{}).Where(whereS, whereP...)
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

func (s *SensitiveDataRepository) Add(ctx *gin.Context, data model.SecuritySensitiveData) error {
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

func (s *SensitiveDataRepository) Edit(ctx *gin.Context, data model.SecuritySensitiveData) error {
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
		result = tx.Model(&model.SecuritySensitiveData{}).Where("id = ?", data.ID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.SecuritySensitiveData{}).Where("id = ?", data.ID).Count(&visible).Error; err != nil {
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

func (s *SensitiveDataRepository) UpdateStatus(ctx *gin.Context, id int32, status string) error {
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		if status == "1" {
			var current model.SecuritySensitiveData
			if err := tx.Where("id = ?", id).First(&current).Error; err != nil {
				return err
			}
			if err := rejectDuplicateEnabledControllerAs(tx, s.config.Database.Prefix+"security_sensitive_data", current.ControllerAs, id); err != nil {
				return err
			}
		}
		result = tx.Model(&model.SecuritySensitiveData{}).Where("id = ?", id).Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.SecuritySensitiveData{}).Where("id = ?", id).Count(&visible).Error; err != nil {
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

func (s *SensitiveDataRepository) Del(ctx *gin.Context, ids interface{}) error {
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
		var list []model.SecuritySensitiveData
		query := tx.Model(&model.SecuritySensitiveData{})
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
