package security

import (
	"fmt"
	adminmodel "go-build-admin/internal/admin/model"
	"go-build-admin/internal/conf"
	"go-build-admin/internal/pkg/data_scope"
	persistence "go-build-admin/internal/pkg/persistence"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SensitiveDataLogModel struct {
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

func NewSensitiveDataLogModel(sqlDB *gorm.DB, config *conf.Configuration) *SensitiveDataLogModel {
	return &SensitiveDataLogModel{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"security_sensitive_data_log", "id", "sensitive.name", sqlDB),
		config:    config,
	}
}

func (s *SensitiveDataLogModel) GetOne(ctx *gin.Context, id int32) (sensitiveData SecuritySensitiveDataLog, err error) {
	prefix := s.config.Database.Prefix
	err = s.DBFor(ctx).Model(&SecuritySensitiveDataLog{}).
		Joins("Admin").
		Joins("SensitiveData").
		Select(sensitiveDataLogSelect(prefix)).
		Where(""+prefix+"security_sensitive_data_log.id=?", id).First(&sensitiveData).Error
	return
}

func (s *SensitiveDataLogModel) List(ctx *gin.Context) (list []SecuritySensitiveDataLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := adminmodel.QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.DBFor(ctx).Model(&SecuritySensitiveDataLog{}).
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

func (s *SensitiveDataLogModel) Rollback(ctx *gin.Context, ids interface{}) error {
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
		condition := "id IN ? AND is_rollback = 0"
		var list []SecuritySensitiveDataLog
		query := tx.Model(&SecuritySensitiveDataLog{})
		if err := query.Where(condition, normalized).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}

		for _, v := range list {
			targetTable, err := data_scope.ResolveBusinessTable(tx, s.config.Database.Prefix, v.DataTable)
			if err != nil || data_scope.ResolveBusinessColumn(tx, targetTable, v.PrimaryKey, s.config.Database.Prefix) != nil || data_scope.ResolveBusinessColumn(tx, targetTable, v.DataField, s.config.Database.Prefix) != nil {
				return fmt.Errorf("invalid sensitive target identifier")
			}
			// Fail-closed: refuse to rollback tables that cannot prove row ownership.
			var rule SecuritySensitiveData
			if err := tx.Table(s.config.Database.Prefix+"security_sensitive_data").Where("id=?", v.SensitiveID).Take(&rule).Error; err != nil {
				return fmt.Errorf("sensitive rule %d unavailable: %w", v.SensitiveID, err)
			}
			if rule.PrimaryKey == "" {
				rule.PrimaryKey = "id"
			}
			if v.PrimaryKey != rule.PrimaryKey {
				return fmt.Errorf("sensitive log primary key does not match historical rule")
			}
			result := tx.Table(targetTable).Where("`"+v.PrimaryKey+"`=? AND `"+v.DataField+"`=?", v.IDValue, v.After).UpdateColumn(v.DataField, v.Before)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			result = tx.Model(&SecuritySensitiveDataLog{}).Where("id = ? AND is_rollback = 0", v.ID).Update("is_rollback", 1)
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

func (s *SensitiveDataLogModel) Del(ctx *gin.Context, ids interface{}) error {
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
		var list []SecuritySensitiveDataLog
		query := tx.Model(&SecuritySensitiveDataLog{})
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
