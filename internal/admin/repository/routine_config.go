package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	siteconfig "buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/conf"
	cErr "buildadmin-go/internal/pkg/error"
	persistence "buildadmin-go/internal/pkg/persistence"
	"gorm.io/gorm"
)

type ConfigRepository struct {
	persistence.BaseModel
	service *siteconfig.Service
}

func NewConfigRepository(sqlDB *gorm.DB, config *conf.Configuration, service *siteconfig.Service) *ConfigRepository {
	if service == nil {
		service = siteconfig.NewService(sqlDB)
	}
	return &ConfigRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"config", "id", "name", sqlDB),
		service:   service,
	}
}

// AllTx loads every config row ordered by weigh inside the caller's
// transaction (the save-all read primitive).
func (s *ConfigRepository) AllTx(tx *gorm.DB) ([]siteconfig.Config, error) {
	all := []siteconfig.Config{}
	if err := tx.Model(&siteconfig.Config{}).Order("`weigh` desc").Find(&all).Error; err != nil {
		return nil, err
	}
	return all, nil
}

// UpdateValueTx writes one config value inside the caller's transaction and
// returns the rows affected (atomic write primitive).
func (s *ConfigRepository) UpdateValueTx(tx *gorm.DB, id int32, value string) (int64, error) {
	result := tx.Table(s.TableName).Where("id=?", id).Update("value", value)
	return result.RowsAffected, result.Error
}

func (s *ConfigRepository) List(ctx *gin.Context) (list []siteconfig.Config, err error) {
	err = s.DBFor(ctx).Model(&siteconfig.Config{}).Order("`weigh` desc").Find(&list).Error
	return
}

func (s *ConfigRepository) Add(ctx context.Context, data siteconfig.Config) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := s.ensureNameAvailable(tx, data.Name, 0); err != nil {
			return err
		}
		result := tx.Create(&data)
		if result.Error != nil {
			if isDuplicateKeyError(result.Error) {
				return cErr.BadRequest("config name already exists")
			}
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("create failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *ConfigRepository) Edit(ctx *gin.Context, data siteconfig.Config) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := s.ensureNameAvailable(tx, data.Name, data.ID); err != nil {
			return err
		}
		result := tx.Save(&data)
		if result.Error != nil {
			if isDuplicateKeyError(result.Error) {
				return cErr.BadRequest("config name already exists")
			}
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *ConfigRepository) ensureNameAvailable(tx *gorm.DB, name string, id int32) error {
	query := tx.Where("name = ?", name)
	if id > 0 {
		query = query.Where("id <> ?", id)
	}
	var existing siteconfig.Config
	err := query.Take(&existing).Error
	switch {
	case err == nil:
		return cErr.BadRequest("config name already exists")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil
	default:
		return err
	}
}


func (s *ConfigRepository) Del(ctx *gin.Context, ids interface{}) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		query := tx.Model(&siteconfig.Config{}).Where("`id` in ? ", ids)
		var expected int64
		if err := query.Count(&expected).Error; err != nil {
			return err
		}
		result := query.Delete(nil)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != expected {
			return fmt.Errorf("delete failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *ConfigRepository) GetOneByName(ctx *gin.Context, name string) (siteconfig.Config, error) {
	var config siteconfig.Config
	err := s.DBFor(ctx).Where("`name`= ? ", name).Take(&config).Error
	return config, err
}

func (s *ConfigRepository) GetValueByName(ctx *gin.Context, name string) (string, error) {
	return s.service.GetValueByName(ctx, name)
}

// 获取键值对模式
func (s *ConfigRepository) GetKVByGroup(ctx *gin.Context, group string) (map[string]string, error) {
	return s.service.GetKVByGroup(ctx, group)
}
