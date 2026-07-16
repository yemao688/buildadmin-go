package model

import (
	"encoding/json"
	"fmt"
	"go-build-admin/app/admin/model/simple"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SecurityDataRecycleLog 数据回收记录表
type SecurityDataRecycleLog struct {
	ID            int32               `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                         // ID
	AdminID       int32               `gorm:"column:admin_id;not null;index:idx_admin_id,priority:1;comment:操作管理员" json:"admin_id"` // 操作管理员
	TargetAdminID int32               `gorm:"column:target_admin_id;not null;index:idx_target_admin_id;comment:目标数据管理员" json:"target_admin_id"`
	RecycleID     int32               `gorm:"column:recycle_id;not null;comment:回收规则ID" json:"recycle_id"`        // 回收规则ID
	Data          string              `gorm:"column:data;comment:回收的数据" json:"data"`                              // 回收的数据
	DataTable     string              `gorm:"column:data_table;not null;comment:数据表" json:"data_table"`           // 数据表
	PrimaryKey    string              `gorm:"column:primary_key;not null;comment:数据表主键" json:"primary_key"`       // 数据表主键
	IsRestore     int32               `gorm:"column:is_restore;not null;comment:是否已还原:0=否,1=是" json:"is_restore"` // 是否已还原:0=否,1=是
	IsCommitted   int32               `gorm:"column:is_committed;not null;default:0" json:"is_committed"`
	Connection    string              `gorm:"column:connection;not null;default:'';comment:数据库连接配置标识" json:"connection"`
	IP            string              `gorm:"column:ip;not null;comment:操作者IP" json:"ip"`                        // 操作者IP
	Useragent     string              `gorm:"column:useragent;not null;comment:User-Agent" json:"useragent"`     // User-Agent
	CreateTime    int64               `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	Admin         simple.Admin        `gorm:"foreignKey:AdminID" json:"admin"`
	Recycle       SecurityDataRecycle `gorm:"foreignKey:RecycleID" json:"recycle"`
}

type DataRecycleLogModel struct {
	BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
}

func NewDataRecycleLogModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *DataRecycleLogModel {
	return &DataRecycleLogModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "security_data_recycle_log",
			Key:              "id",
			QuickSearchField: "recycle.name",
			sqlDB:            sqlDB,
		},
		config:   config,
		enforcer: enforcer,
	}
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// security_data_recycle_log.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *DataRecycleLogModel) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
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
func (s *DataRecycleLogModel) targetHasAdminID(tx *gorm.DB, dataTable string) (bool, error) {
	var count int64
	err := tx.Raw(
		"SELECT COUNT(*) FROM information_schema.columns "+
			"WHERE table_schema = DATABASE() AND table_name = ? AND column_name = 'admin_id'",
		s.config.Database.Prefix+dataTable,
	).Scan(&count).Error
	return count > 0, err
}

func (s *DataRecycleLogModel) hasLegacyUnrecoverable(tx *gorm.DB) (bool, error) {
	var count int64
	err := tx.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name='legacy_unrecoverable'", s.TableName).Scan(&count).Error
	return count == 1, err
}

func (s *DataRecycleLogModel) GetOne(ctx *gin.Context, id int32) (dataRecycle SecurityDataRecycleLog, err error) {
	prefix := s.config.Database.Prefix
	err = s.DBFor(ctx).Model(&SecurityDataRecycleLog{}).
		Scopes(s.scoped(ctx)).
		Preload("Admin").
		Preload("Recycle").
		Joins("left join "+prefix+"admin admin on admin.id = "+prefix+"security_data_recycle_log.admin_id").
		Joins("left join "+prefix+"security_data_recycle recycle on recycle.id = "+prefix+"security_data_recycle_log.recycle_id").Where(""+prefix+"security_data_recycle_log.id=?", id).First(&dataRecycle).Error
	return
}

func (s *DataRecycleLogModel) List(ctx *gin.Context) (list []SecurityDataRecycleLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	prefix := s.config.Database.Prefix
	db := s.DBFor(ctx).Model(&SecurityDataRecycleLog{}).
		Scopes(s.scoped(ctx)).
		Preload("Admin").
		Preload("Recycle").
		Joins("left join "+prefix+"admin admin on admin.id = "+prefix+"security_data_recycle_log.admin_id").
		Joins("left join "+prefix+"security_data_recycle recycle on recycle.id = "+prefix+"security_data_recycle_log.recycle_id").
		Where(whereS, whereP...)

	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *DataRecycleLogModel) Restore(ctx *gin.Context, ids interface{}) error {
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
		legacy, err := s.hasLegacyUnrecoverable(tx)
		if err != nil {
			return err
		}
		condition := "id IN ? AND is_restore = 0 AND is_committed = 1"
		if legacy {
			condition += " AND legacy_unrecoverable = 0"
		}
		var list []SecurityDataRecycleLog
		scoped := tx.Model(&SecurityDataRecycleLog{}).Scopes(s.scoped(ctx))
		if err := scoped.Where(condition, normalized).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}

		for _, v := range list {
			targetTable, err := data_scope.ResolveBusinessTable(tx, s.config.Database.Prefix, v.DataTable)
			if err != nil || data_scope.ResolveBusinessColumn(tx, targetTable, v.PrimaryKey) != nil {
				return fmt.Errorf("invalid recycle target identifier")
			}
			data := map[string]any{}
			if err := json.Unmarshal([]byte(v.Data), &data); err != nil {
				return err
			}

			//gorm scan到map会包含时间,还原时需要去掉时间
			if v.DataTable == "user" && data["birthday"] != "" {
				data["birthday"] = data["birthday"].(string)[:10]
			}

			// Fail-closed: refuse to restore into tables that cannot carry ownership.
			hasAdminID, err := s.targetHasAdminID(tx, v.DataTable)
			if err != nil {
				return err
			}
			if !hasAdminID {
				return fmt.Errorf("target table %s lacks admin_id, cannot restore safely", v.DataTable)
			}
			if v.TargetAdminID <= 0 {
				return fmt.Errorf("recycle log %d has no target owner", v.ID)
			}
			if err := data_scope.OwnerInScope(ctx, tx, s.enforcer, s.config.Database.Prefix, v.TargetAdminID); err != nil {
				return err
			}
			data["admin_id"] = v.TargetAdminID

			if err := tx.Table(targetTable).Create(data).Error; err != nil {
				return err
			}
			result := tx.Model(&SecurityDataRecycleLog{}).Scopes(s.scoped(ctx)).Where("id = ? AND is_restore = 0", v.ID).Update("is_restore", 1)
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

func (s *DataRecycleLogModel) Del(ctx *gin.Context, ids interface{}) error {
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
		var list []SecurityDataRecycleLog
		scoped := tx.Model(&SecurityDataRecycleLog{}).Scopes(s.scoped(ctx))
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
