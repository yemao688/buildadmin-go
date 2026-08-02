package security

import (
	"encoding/json"
	"fmt"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	persistence "buildadmin-go/internal/pkg/persistence"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DataRecycleLogRepository struct {
	persistence.BaseModel
	config *conf.Configuration
}

func dataRecycleLogSelect(prefix string) string {
	return strings.Join([]string{
		prefix + "security_data_recycle_log.*",
		"Admin.id AS Admin__id", "Admin.username AS Admin__username", "Admin.nickname AS Admin__nickname",
		"Recycle.id AS Recycle__id", "Recycle.name AS Recycle__name", "Recycle.controller AS Recycle__controller",
		"Recycle.controller_as AS Recycle__controller_as", "Recycle.data_table AS Recycle__data_table",
		"Recycle.primary_key AS Recycle__primary_key", "Recycle.status AS Recycle__status",
		"Recycle.connection AS Recycle__connection", "Recycle.update_time AS Recycle__update_time",
		"Recycle.create_time AS Recycle__create_time",
	}, ", ")
}

func NewDataRecycleLogRepository(sqlDB *gorm.DB, config *conf.Configuration) *DataRecycleLogRepository {
	return &DataRecycleLogRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"security_data_recycle_log", "id", "recycle.name", sqlDB),
		config:    config,
	}
}

func (s *DataRecycleLogRepository) GetOne(ctx *gin.Context, id int32) (dataRecycle model.SecurityDataRecycleLog, err error) {
	prefix := s.config.Database.Prefix
	err = s.DBFor(ctx).Model(&model.SecurityDataRecycleLog{}).
		Joins("Admin").
		Joins("Recycle").
		Select(dataRecycleLogSelect(prefix)).
		Where(""+prefix+"security_data_recycle_log.id=?", id).First(&dataRecycle).Error
	return
}

func (s *DataRecycleLogRepository) List(ctx *gin.Context) (list []model.SecurityDataRecycleLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := adminmodel.QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.DBFor(ctx).Model(&model.SecurityDataRecycleLog{}).
		Joins("Admin").
		Joins("Recycle").
		Select(dataRecycleLogSelect(s.config.Database.Prefix)).
		Where(whereS, whereP...)

	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *DataRecycleLogRepository) Restore(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid recycle log ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid recycle log id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		condition := "id IN ? AND is_restore = 0"
		var list []model.SecurityDataRecycleLog
		query := tx.Model(&model.SecurityDataRecycleLog{})
		if err := query.Where(condition, normalized).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}

		for _, v := range list {
			targetTable, err := data_scope.ResolveBusinessTable(tx, s.config.Database.Prefix, v.DataTable)
			if err != nil || data_scope.ResolveBusinessColumn(tx, targetTable, v.PrimaryKey, s.config.Database.Prefix) != nil {
				return fmt.Errorf("invalid recycle target identifier")
			}
			data := map[string]any{}
			if err := json.Unmarshal([]byte(v.Data), &data); err != nil {
				return err
			}

			// Fail-closed: refuse to restore into tables that cannot carry ownership.
			var rule model.SecurityDataRecycle
			if err := tx.Table(s.config.Database.Prefix+"security_data_recycle").Where("id=?", v.RecycleID).Take(&rule).Error; err != nil {
				return fmt.Errorf("recycle rule %d unavailable: %w", v.RecycleID, err)
			}
			if rule.PrimaryKey == "" {
				rule.PrimaryKey = "id"
			}
			if v.PrimaryKey != rule.PrimaryKey {
				return fmt.Errorf("recycle log primary key does not match historical rule")
			}
			if err := tx.Table(targetTable).Create(data).Error; err != nil {
				return err
			}
			result := tx.Model(&model.SecurityDataRecycleLog{}).Where("id = ? AND is_restore = 0", v.ID).Update("is_restore", 1)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
}

func (s *DataRecycleLogRepository) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid recycle log ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid recycle log id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var list []model.SecurityDataRecycleLog
		query := tx.Model(&model.SecurityDataRecycleLog{})
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
