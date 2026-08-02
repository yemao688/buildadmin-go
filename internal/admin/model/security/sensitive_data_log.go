package security

import (
	"fmt"
	adminmodel "go-build-admin/internal/admin/model"
	"go-build-admin/internal/admin/model/simple"
	"go-build-admin/internal/pkg/data_scope"
	"go-build-admin/internal/conf"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SecuritySensitiveDataLog 敏感数据修改记录
type SecuritySensitiveDataLog struct {
	ID            int32                 `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                         // ID
	AdminID       int32                 `gorm:"column:admin_id;not null;index:idx_admin_id,priority:1;comment:操作管理员" json:"admin_id"` // 操作管理员
	SensitiveID   int32                 `gorm:"column:sensitive_id;not null;comment:敏感数据规则ID" json:"sensitive_id"`                    // 敏感数据规则ID
	DataTable     string                `gorm:"column:data_table;not null;comment:数据表" json:"data_table"`                             // 数据表
	PrimaryKey    string                `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`                         // 数据表主键
	DataField     string                `gorm:"column:data_field;not null;comment:被修改字段" json:"data_field"`                           // 被修改字段
	DataComment   string                `gorm:"column:data_comment;not null;comment:被修改项" json:"data_comment"`                        // 被修改项
	IDValue       int32                 `gorm:"column:id_value;not null;comment:被修改项主键值" json:"id_value"`                             // 被修改项主键值
	Before        string                `gorm:"column:before;comment:修改前" json:"before"`                                              // 修改前
	After         string                `gorm:"column:after;comment:修改后" json:"after"`                                                // 修改后
	IP            string                `gorm:"column:ip;not null;comment:操作者IP" json:"ip"`                                           // 操作者IP
	Useragent     string                `gorm:"column:useragent;not null;comment:User-Agent" json:"useragent"`                        // User-Agent
	IsRollback    int32                 `gorm:"column:is_rollback;not null;comment:是否已回滚:0=否,1=是" json:"is_rollback"`                 // 是否已回滚:0=否,1=是
	Connection    string                `gorm:"column:connection;not null;default:'';comment:数据库连接配置标识" json:"connection"`
	CreateTime    int64                 `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	Admin         simple.Admin          `gorm:"foreignKey:AdminID" json:"admin"`
	SensitiveData SecuritySensitiveData `gorm:"foreignKey:SensitiveID" json:"sensitive"`
}

type SensitiveDataLogModel struct {
	adminmodel.BaseModel
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
		BaseModel: adminmodel.NewBaseModel(config.Database.Prefix+"security_sensitive_data_log", "id", "sensitive.name", sqlDB),
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
