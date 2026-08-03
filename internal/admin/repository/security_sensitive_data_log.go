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

type SecuritySensitiveDataLogRepository struct {
	persistence.BaseModel
	config *conf.Configuration
}

func sensitiveDataLogSelect(prefix string) string {
	return strings.Join([]string{
		prefix + "security_sensitive_data_log.*",
		"Admin.id AS Admin__id", "Admin.username AS Admin__username", "Admin.nickname AS Admin__nickname",
		"SensitiveData.id AS SensitiveData__id", "SensitiveData.name AS SensitiveData__name",
		"SensitiveData.controller AS SensitiveData__controller", "SensitiveData.controller_as AS SensitiveData__controller_as",
		"SensitiveData.data_table AS SensitiveData__data_table", "SensitiveData.primary_key AS SensitiveData__primary_key",
		"SensitiveData.data_fields AS SensitiveData__data_fields", "SensitiveData.status AS SensitiveData__status",
		"SensitiveData.connection AS SensitiveData__connection", "SensitiveData.update_time AS SensitiveData__update_time",
		"SensitiveData.create_time AS SensitiveData__create_time",
	}, ", ")
}

func NewSecuritySensitiveDataLogRepository(sqlDB *gorm.DB, config *conf.Configuration) *SecuritySensitiveDataLogRepository {
	return &SecuritySensitiveDataLogRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"security_sensitive_data_log", "id", "sensitive.name", sqlDB),
		config:    config,
	}
}

func (s *SecuritySensitiveDataLogRepository) GetOne(ctx *gin.Context, id int32) (sensitiveData model.SecuritySensitiveDataLog, err error) {
	prefix := s.config.Database.Prefix
	err = s.DBFor(ctx).Model(&model.SecuritySensitiveDataLog{}).
		Joins("Admin").
		Joins("SensitiveData").
		Select(sensitiveDataLogSelect(prefix)).
		Where(""+prefix+"security_sensitive_data_log.id=?", id).First(&sensitiveData).Error
	return
}

func (s *SecuritySensitiveDataLogRepository) List(ctx *gin.Context) (list []model.SecuritySensitiveDataLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.DBFor(ctx).Model(&model.SecuritySensitiveDataLog{}).
		Joins("Admin").
		Joins("SensitiveData").
		Select(sensitiveDataLogSelect(s.config.Database.Prefix)).
		Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

// LockPendingLogs loads the non-rolled-back logs FOR UPDATE; a missing or
// already-processed id fails with gorm.ErrRecordNotFound so the batch is
// all-or-nothing.
func (s *SecuritySensitiveDataLogRepository) LockPendingLogs(tx *gorm.DB, ids []int32) ([]model.SecuritySensitiveDataLog, error) {
	condition := "id IN ? AND is_rollback = 0"
	var list []model.SecuritySensitiveDataLog
	query := tx.Model(&model.SecuritySensitiveDataLog{})
	if err := query.Where(condition, ids).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) != len(ids) {
		return nil, gorm.ErrRecordNotFound
	}
	return list, nil
}

// ResolveTarget validates the target table and its primary/data columns for
// a sensitive log entry (fail-closed identifier resolution).
func (s *SecuritySensitiveDataLogRepository) ResolveTarget(tx *gorm.DB, dataTable, primaryKey, dataField string) (string, error) {
	targetTable, err := data_scope.ResolveBusinessTable(tx, s.config.Database.Prefix, dataTable)
	if err != nil || data_scope.ResolveBusinessColumn(tx, targetTable, primaryKey, s.config.Database.Prefix) != nil || data_scope.ResolveBusinessColumn(tx, targetTable, dataField, s.config.Database.Prefix) != nil {
		return "", fmt.Errorf("invalid sensitive target identifier")
	}
	return targetTable, nil
}

// SensitiveRuleByIDTx loads the historical rule that governs a log entry.
func (s *SecuritySensitiveDataLogRepository) SensitiveRuleByIDTx(tx *gorm.DB, id int32) (model.SecuritySensitiveData, error) {
	var rule model.SecuritySensitiveData
	if err := tx.Table(s.config.Database.Prefix + "security_sensitive_data").Where("id=?", id).Take(&rule).Error; err != nil {
		return rule, err
	}
	return rule, nil
}

// ApplyFieldRestoreTx writes the historical value back inside the caller's
// transaction, verifying the current value still matches (optimistic guard).
func (s *SecuritySensitiveDataLogRepository) ApplyFieldRestoreTx(tx *gorm.DB, targetTable, primaryKey string, idValue int32, dataField, before, after string) error {
	result := tx.Table(targetTable).Where("`"+primaryKey+"`=? AND `"+dataField+"`=?", idValue, after).UpdateColumn(dataField, before)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// MarkRolledBackTx flips the rollback flag inside the caller's transaction.
func (s *SecuritySensitiveDataLogRepository) MarkRolledBackTx(tx *gorm.DB, id int32) error {
	result := tx.Model(&model.SecuritySensitiveDataLog{}).Where("id = ? AND is_rollback = 0", id).Update("is_rollback", 1)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *SecuritySensitiveDataLogRepository) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid sensitive data log ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid sensitive data log id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var list []model.SecuritySensitiveDataLog
		query := tx.Model(&model.SecuritySensitiveDataLog{})
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
