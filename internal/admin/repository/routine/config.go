package routine

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	mysql "github.com/go-sql-driver/mysql"
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

// SaveAll persists submitted config values in one transaction: every known
// config row whose name appears in params is updated (weigh-ordered to match
// the historical write order); empty upload_secret_key submissions are skipped
// so the stored secret is never blanked by a form round-trip.
func (s *ConfigRepository) SaveAll(ctx *gin.Context, params map[string]interface{}) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		all := []siteconfig.Config{}
		if err := tx.Model(&siteconfig.Config{}).Order("`weigh` desc").Find(&all).Error; err != nil {
			return err
		}
		for _, v := range all {
			value, ok := params[v.Name]
			if !ok {
				continue
			}
			if v.Name == "upload_secret_key" && fmt.Sprintf("%v", value) == "" {
				continue
			}
			newValue := v.SetValueAttr(value, v.Type)
			if err := updateConfigValue(v.Value, newValue, func() (int64, error) {
				result := tx.Table(s.TableName).Where("id=?", v.ID).Update("value", newValue)
				return result.RowsAffected, result.Error
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// updateConfigValue applies one config value change, rejecting silent no-ops
// so a lost row surfaces instead of passing as a successful save.
func updateConfigValue(currentValue, newValue string, update func() (int64, error)) error {
	if newValue == currentValue {
		return nil
	}

	rowsAffected, err := update()
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return fmt.Errorf("config update failed: rows affected mismatch")
	}
	return nil
}

func (s *ConfigRepository) List(ctx *gin.Context) (list []siteconfig.Config, err error) {
	err = s.DBFor(ctx).Model(&siteconfig.Config{}).Order("`weigh` desc").Find(&list).Error
	return
}

func (s *ConfigRepository) Add(ctx *gin.Context, data siteconfig.Config) error {
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

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
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
