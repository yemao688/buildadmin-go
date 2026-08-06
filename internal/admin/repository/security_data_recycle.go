package repository

import (
	"fmt"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	persistence "buildadmin-go/internal/pkg/persistence"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SecurityDataRecycleRepository struct {
	persistence.BaseModel
	config *conf.Configuration
}

func NewSecurityDataRecycleRepository(sqlDB *gorm.DB, config *conf.Configuration) *SecurityDataRecycleRepository {
	return &SecurityDataRecycleRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"security_data_recycle", "id", "name", sqlDB),
		config:    config,
	}
}

func (s *SecurityDataRecycleRepository) GetOne(ctx *gin.Context, id int32) (data model.SecurityDataRecycle, err error) {
	err = s.DBFor(ctx).Model(&model.SecurityDataRecycle{}).Where("id=?", id).First(&data).Error
	return
}

func (s *SecurityDataRecycleRepository) List(ctx *gin.Context) (list []model.SecurityDataRecycle, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	// count 与 find 必须使用独立 statement：复用同一 db 先 Count 再 Find 时，
	// GORM 的 Count 会重置 statement，导致 Find 丢失搜索 WHERE。
	countDB := s.DBFor(ctx).Model(&model.SecurityDataRecycle{}).Where(whereS, whereP...)
	if err = countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	findDB := s.DBFor(ctx).Model(&model.SecurityDataRecycle{}).Where(whereS, whereP...)
	err = findDB.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

// GetByIDTx loads one rule inside the caller's transaction.
func (s *SecurityDataRecycleRepository) GetByIDTx(tx *gorm.DB, id int32) (model.SecurityDataRecycle, error) {
	var current model.SecurityDataRecycle
	err := tx.Where("id = ?", id).First(&current).Error
	return current, err
}

// ResolvePolicyTx validates the target table/column policy for a security
// rule inside the caller's transaction (data-access validation).
func (s *SecurityDataRecycleRepository) ResolvePolicyTx(tx *gorm.DB, logical, kind, primary string, fields []string) (data_scope.RulePolicy, error) {
	return resolveRulePolicy(tx, s.config.Database.Prefix, logical, kind, primary, fields)
}

// EnabledControllerAsCount counts enabled rules that already use the
// controller_as (the duplicate-guard data source).
func (s *SecurityDataRecycleRepository) EnabledControllerAsCount(tx *gorm.DB, controllerAs string, excludeID int32) (int64, error) {
	query := tx.Table(s.config.Database.Prefix+"security_data_recycle").Where("status = ? AND controller_as = ?", "1", controllerAs)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CreateTx inserts one rule inside the caller's transaction (atomic write
// primitive).
func (s *SecurityDataRecycleRepository) CreateTx(tx *gorm.DB, data *model.SecurityDataRecycle) error {
	return tx.Create(data).Error
}

// UpdateTx applies the rule updates inside the caller's transaction,
// verifying RowsAffected so a lost row surfaces instead of passing silently.
func (s *SecurityDataRecycleRepository) UpdateTx(tx *gorm.DB, id int32, updates map[string]any) error {
	result := tx.Model(&model.SecurityDataRecycle{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var visible int64
		if err := tx.Model(&model.SecurityDataRecycle{}).Where("id = ?", id).Count(&visible).Error; err != nil {
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
}

// UpdateStatusTx applies a status switch inside the caller's transaction,
// verifying RowsAffected.
func (s *SecurityDataRecycleRepository) UpdateStatusTx(tx *gorm.DB, id int32, status string) error {
	result := tx.Model(&model.SecurityDataRecycle{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var visible int64
		if err := tx.Model(&model.SecurityDataRecycle{}).Where("id = ?", id).Count(&visible).Error; err != nil {
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
}

func (s *SecurityDataRecycleRepository) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid security data recycle ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid security data recycle id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var list []model.SecurityDataRecycle
		query := tx.Model(&model.SecurityDataRecycle{})
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
