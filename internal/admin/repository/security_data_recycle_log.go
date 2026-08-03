package repository

import (
	"fmt"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	persistence "buildadmin-go/internal/pkg/persistence"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SecurityDataRecycleLogRepository struct {
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

func NewSecurityDataRecycleLogRepository(sqlDB *gorm.DB, config *conf.Configuration) *SecurityDataRecycleLogRepository {
	return &SecurityDataRecycleLogRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"security_data_recycle_log", "id", "recycle.name", sqlDB),
		config:    config,
	}
}

func (s *SecurityDataRecycleLogRepository) GetOne(ctx *gin.Context, id int32) (dataRecycle model.SecurityDataRecycleLog, err error) {
	prefix := s.config.Database.Prefix
	err = s.DBFor(ctx).Model(&model.SecurityDataRecycleLog{}).
		Joins("Admin").
		Joins("Recycle").
		Select(dataRecycleLogSelect(prefix)).
		Where(""+prefix+"security_data_recycle_log.id=?", id).First(&dataRecycle).Error
	return
}

func (s *SecurityDataRecycleLogRepository) List(ctx *gin.Context) (list []model.SecurityDataRecycleLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
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

// LockPendingLogs loads the non-restored logs FOR UPDATE; a missing or
// already-processed id fails with gorm.ErrRecordNotFound so the batch is
// all-or-nothing.
func (s *SecurityDataRecycleLogRepository) LockPendingLogs(tx *gorm.DB, ids []int32) ([]model.SecurityDataRecycleLog, error) {
	condition := "id IN ? AND is_restore = 0"
	var list []model.SecurityDataRecycleLog
	query := tx.Model(&model.SecurityDataRecycleLog{})
	if err := query.Where(condition, ids).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) != len(ids) {
		return nil, gorm.ErrRecordNotFound
	}
	return list, nil
}

// ResolveTarget validates the target table and its primary column for a
// recycle log entry (fail-closed identifier resolution).
func (s *SecurityDataRecycleLogRepository) ResolveTarget(tx *gorm.DB, dataTable, primaryKey string) (string, error) {
	targetTable, err := data_scope.ResolveBusinessTable(tx, s.config.Database.Prefix, dataTable)
	if err != nil || data_scope.ResolveBusinessColumn(tx, targetTable, primaryKey, s.config.Database.Prefix) != nil {
		return "", fmt.Errorf("invalid recycle target identifier")
	}
	return targetTable, nil
}

// RecycleRuleByIDTx loads the historical rule that governs a log entry.
func (s *SecurityDataRecycleLogRepository) RecycleRuleByIDTx(tx *gorm.DB, id int32) (model.SecurityDataRecycle, error) {
	var rule model.SecurityDataRecycle
	if err := tx.Table(s.config.Database.Prefix + "security_data_recycle").Where("id=?", id).Take(&rule).Error; err != nil {
		return rule, err
	}
	return rule, nil
}

// CreateRowTx inserts the restored row inside the caller's transaction.
func (s *SecurityDataRecycleLogRepository) CreateRowTx(tx *gorm.DB, targetTable string, data map[string]any) error {
	return tx.Table(targetTable).Create(data).Error
}

// MarkRestoredTx flips the restore flag inside the caller's transaction.
func (s *SecurityDataRecycleLogRepository) MarkRestoredTx(tx *gorm.DB, id int32) error {
	result := tx.Model(&model.SecurityDataRecycleLog{}).Where("id = ? AND is_restore = 0", id).Update("is_restore", 1)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *SecurityDataRecycleLogRepository) Del(ctx *gin.Context, ids interface{}) error {
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
