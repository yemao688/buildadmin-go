package model

import (
	"fmt"
	"go-build-admin/app/admin/model/simple"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SecuritySensitiveDataLog 敏感数据修改记录
type SecuritySensitiveDataLog struct {
	ID            int32                 `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                         // ID
	AdminID       int32                 `gorm:"column:admin_id;not null;index:idx_admin_id,priority:1;comment:操作管理员" json:"admin_id"` // 操作管理员
	TargetAdminID int32                 `gorm:"column:target_admin_id;not null;index:idx_target_admin_id;comment:目标数据管理员" json:"target_admin_id"`
	SensitiveID   int32                 `gorm:"column:sensitive_id;not null;comment:敏感数据规则ID" json:"sensitive_id"`    // 敏感数据规则ID
	DataTable     string                `gorm:"column:data_table;not null;comment:数据表" json:"data_table"`             // 数据表
	PrimaryKey    string                `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`         // 数据表主键
	DataField     string                `gorm:"column:data_field;not null;comment:被修改字段" json:"data_field"`           // 被修改字段
	DataComment   string                `gorm:"column:data_comment;not null;comment:被修改项" json:"data_comment"`        // 被修改项
	IDValue       int32                 `gorm:"column:id_value;not null;comment:被修改项主键值" json:"id_value"`             // 被修改项主键值
	Before        string                `gorm:"column:before;comment:修改前" json:"before"`                              // 修改前
	After         string                `gorm:"column:after;comment:修改后" json:"after"`                                // 修改后
	IP            string                `gorm:"column:ip;not null;comment:操作者IP" json:"ip"`                           // 操作者IP
	Useragent     string                `gorm:"column:useragent;not null;comment:User-Agent" json:"useragent"`        // User-Agent
	IsRollback    int32                 `gorm:"column:is_rollback;not null;comment:是否已回滚:0=否,1=是" json:"is_rollback"` // 是否已回滚:0=否,1=是
	IsCommitted   int32                 `gorm:"column:is_committed;not null;default:0" json:"is_committed"`
	Connection    string                `gorm:"column:connection;not null;default:'';comment:数据库连接配置标识" json:"connection"`
	CreateTime    int64                 `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	Admin         simple.Admin          `gorm:"foreignKey:AdminID" json:"admin"`
	SensitiveData SecuritySensitiveData `gorm:"foreignKey:SensitiveID" json:"sensitive"`
}

type SensitiveDataLogModel struct {
	BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
}

func NewSensitiveDataLogModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *SensitiveDataLogModel {
	return &SensitiveDataLogModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "security_sensitive_data_log",
			Key:              "id",
			QuickSearchField: "sensitive.name",
			sqlDB:            sqlDB,
		},
		config:   config,
		enforcer: enforcer,
	}
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// security_sensitive_data_log.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *SensitiveDataLogModel) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

// targetHasAdminID reports whether the named target table has an admin_id
// column. It is used to fail-closed restore/rollback operations that would
// otherwise modify rows whose owner cannot be determined.
func (s *SensitiveDataLogModel) targetHasAdminID(tx *gorm.DB, dataTable string) (bool, error) {
	var count int64
	err := tx.Raw(
		"SELECT COUNT(*) FROM information_schema.columns "+
			"WHERE table_schema = DATABASE() AND table_name = ? AND column_name = 'admin_id'",
		s.config.Database.Prefix+dataTable,
	).Scan(&count).Error
	return count > 0, err
}

func (s *SensitiveDataLogModel) hasLegacyUnrecoverable(tx *gorm.DB) (bool, error) {
	var count int64
	err := tx.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='legacy_unrecoverable'", s.TableName).Scan(&count).Error
	return count == 1, err
}

func (s *SensitiveDataLogModel) GetOne(ctx *gin.Context, id int32) (sensitiveData SecuritySensitiveDataLog, err error) {
	prefix := s.config.Database.Prefix
	err = s.DBFor(ctx).Model(&SecuritySensitiveDataLog{}).
		Scopes(s.scoped(ctx)).
		Preload("Admin").
		Preload("SensitiveData").
		Joins("left join "+prefix+"admin admin on admin.id = "+prefix+"security_sensitive_data_log.admin_id").
		Joins("left join "+prefix+"security_sensitive_data sensitive_data on sensitive_data.id = "+prefix+"security_sensitive_data_log.sensitive_id").Where(""+prefix+"security_sensitive_data_log.id=?", id).First(&sensitiveData).Error
	return
}

func (s *SensitiveDataLogModel) List(ctx *gin.Context) (list []SecuritySensitiveDataLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	prefix := s.config.Database.Prefix
	db := s.DBFor(ctx).Model(&SecuritySensitiveDataLog{}).
		Scopes(s.scoped(ctx)).
		Preload("Admin").
		Preload("SensitiveData").
		Joins("left join "+prefix+"admin admin on admin.id = "+prefix+"security_sensitive_data_log.admin_id").
		Joins("left join "+prefix+"security_sensitive_data sensitive_data on sensitive_data.id = "+prefix+"security_sensitive_data_log.sensitive_id").Where(whereS, whereP...)
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
		legacy, err := s.hasLegacyUnrecoverable(tx)
		if err != nil {
			return err
		}
		condition := "id IN ? AND is_rollback = 0 AND is_committed = 1"
		if legacy {
			condition += " AND legacy_unrecoverable = 0"
		}
		var list []SecuritySensitiveDataLog
		scoped := tx.Model(&SecuritySensitiveDataLog{}).Scopes(s.scoped(ctx))
		if err := scoped.Where(condition, normalized).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}

		for _, v := range list {
			targetTable, err := data_scope.ResolveBusinessTable(tx, s.config.Database.Prefix, v.DataTable)
			if err != nil || data_scope.ResolveBusinessColumn(tx, targetTable, v.PrimaryKey) != nil || data_scope.ResolveBusinessColumn(tx, targetTable, v.DataField) != nil {
				return fmt.Errorf("invalid sensitive target identifier")
			}
			// Fail-closed: refuse to rollback tables that cannot prove row ownership.
			hasAdminID, err := s.targetHasAdminID(tx, v.DataTable)
			if err != nil {
				return err
			}
			if !hasAdminID {
				return fmt.Errorf("target table %s lacks admin_id, cannot rollback safely", v.DataTable)
			}

			var rowAdminID int32
			targetScoped := s.enforcer.Scope(ctx, tx.Table(targetTable), data_scope.OwnerRef{TableAlias: targetTable, Column: "admin_id"})
			if err := targetScoped.Select("admin_id").Where("`"+v.PrimaryKey+"`=?", v.IDValue).Take(&rowAdminID).Error; err != nil {
				return err
			}
			if v.TargetAdminID <= 0 || rowAdminID != v.TargetAdminID {
				return fmt.Errorf("target row owner mismatch")
			}
			if err := data_scope.OwnerInScope(ctx, tx, s.enforcer, s.config.Database.Prefix, v.TargetAdminID); err != nil {
				return err
			}

			// Use a fresh scoped session for the conditional write. Reusing the
			// session after the ownership probe can retain the probe statement's
			// RowsAffected value and make a skipped UPDATE look successful.
			targetUpdate := s.enforcer.Scope(ctx, tx.Table(targetTable), data_scope.OwnerRef{TableAlias: targetTable, Column: "admin_id"})
			result := targetUpdate.Where("`"+v.PrimaryKey+"`=? AND admin_id=? AND `"+v.DataField+"`=?", v.IDValue, v.TargetAdminID, v.After).UpdateColumn(v.DataField, v.Before)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			result = tx.Model(&SecuritySensitiveDataLog{}).Scopes(s.scoped(ctx)).Where("id = ? AND is_rollback = 0", v.ID).Update("is_rollback", 1)
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
		scoped := tx.Model(&SecuritySensitiveDataLog{}).Scopes(s.scoped(ctx))
		if err := scoped.Where("id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		del := scoped.Where("id IN ?", normalized).Delete(nil)
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
