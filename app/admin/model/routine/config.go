package routine

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	mysql "github.com/go-sql-driver/mysql"
	model "go-build-admin/app/admin/model"
	siteconfig "go-build-admin/app/common/siteconfig"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/conf"
	"gorm.io/gorm"
)

type ConfigModel struct {
	model.BaseModel
	service *siteconfig.Service
}

func NewConfigModel(sqlDB *gorm.DB, config *conf.Configuration, service *siteconfig.Service) *ConfigModel {
	if service == nil {
		service = siteconfig.NewService(sqlDB)
	}
	return &ConfigModel{
		BaseModel: model.NewBaseModel(config.Database.Prefix+"config", "id", "name", sqlDB),
		service:   service,
	}
}

func (s *ConfigModel) List(ctx *gin.Context) (list []siteconfig.Config, err error) {
	err = s.DBFor(ctx).Model(&siteconfig.Config{}).Order("`weigh` desc").Find(&list).Error
	return
}

func (s *ConfigModel) Add(ctx *gin.Context, data siteconfig.Config) error {
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

func (s *ConfigModel) Edit(ctx *gin.Context, data siteconfig.Config) error {
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

func (s *ConfigModel) ensureNameAvailable(tx *gorm.DB, name string, id int32) error {
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

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func (s *ConfigModel) Del(ctx *gin.Context, ids interface{}) error {
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

func (s *ConfigModel) GetOneByName(ctx *gin.Context, name string) (siteconfig.Config, error) {
	var config siteconfig.Config
	err := s.DBFor(ctx).Where("`name`= ? ", name).Take(&config).Error
	return config, err
}

func (s *ConfigModel) GetValueByName(ctx *gin.Context, name string) (string, error) {
	return s.service.GetValueByName(ctx, name)
}

// 获取键值对模式
func (s *ConfigModel) GetKVByGroup(ctx *gin.Context, group string) (map[string]string, error) {
	return s.service.GetKVByGroup(ctx, group)
}
