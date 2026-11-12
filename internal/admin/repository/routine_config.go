package repository

import (
	siteconfig "buildadmin-go/internal/common/siteconfig"
	"buildadmin-go/internal/conf"
	cErr "buildadmin-go/internal/pkg/error"
	persistence "buildadmin-go/internal/pkg/persistence"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
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

// NameExists reports whether any config row already owns name. excludeID > 0
// ignores that row (the edit-path shape); pass 0 when creating. Name uniqueness
// is a service precheck; Add/Edit keep the duplicate-key fallback for a
// concurrent insert.
func (s *ConfigRepository) NameExists(ctx context.Context, name string, excludeID int32) (bool, error) {
	query := s.DBFor(ctx).Model(&siteconfig.Config{}).Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *ConfigRepository) Add(ctx context.Context, data siteconfig.Config) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
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

// Edit saves one config row. It is transport-free so the service can call it
// after its own name precheck; the duplicate-key fallback covers a concurrent
// insert.
func (s *ConfigRepository) Edit(ctx context.Context, data siteconfig.Config) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		result := tx.Save(&data)
		if result.Error != nil {
			if isDuplicateKeyError(result.Error) {
				return cErr.BadRequest("config name already exists")
			}
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&siteconfig.Config{}).Where("id = ?", data.ID).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return fmt.Errorf("update failed: rows affected mismatch")
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("update failed: rows affected mismatch")
		}
		return nil
	})
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

// GetValueByName 读取单个配置项的值（薄转发到 siteconfig 服务，ctx 携带
// 请求事务时读取事务连接）。
func (s *ConfigRepository) GetValueByName(ctx context.Context, name string) (string, error) {
	return s.service.GetValueByName(ctx, name)
}

// GetKVByGroup 获取键值对模式（薄转发到 siteconfig 服务）。
func (s *ConfigRepository) GetKVByGroup(ctx context.Context, group string) (map[string]string, error) {
	return s.service.GetKVByGroup(ctx, group)
}
