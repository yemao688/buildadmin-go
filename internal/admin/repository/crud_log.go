package repository

import (
	"fmt"
	"strings"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	persistence "buildadmin-go/internal/pkg/persistence"
	querybuilder "buildadmin-go/internal/pkg/querybuilder"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CrudLogRepository 提供 crud_log 表的仓储访问（数据权限受 enforcer 约束）。
// 由 internal/model/crud_log.go 的 LogModel 仓库形态平移而来（C1）。
type CrudLogRepository struct {
	persistence.BaseModel
	enforcer data_scope.Enforcer
}

func NewCrudLogRepository(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *CrudLogRepository {
	return &CrudLogRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"crud_log", "id", "table_name", sqlDB),
		enforcer:  enforcer,
	}
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// crud_log.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *CrudLogRepository) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

func (s *CrudLogRepository) GetByTableName(ctx *gin.Context, table string) (crudLog model.Log, err error) {
	err = s.DBFor(ctx).Model(&model.Log{}).Scopes(s.scoped(ctx)).Where("table_name=?", table).Order("create_time desc").Take(&crudLog).Error
	return
}

// HasAnyByTableName intentionally bypasses data scope. It is used only to
// distinguish a first generation from regeneration; exposing the log row is
// not required. A scoped lookup would let another administrator overwrite a
// handwritten file merely because somebody else generated that table.
func (s *CrudLogRepository) HasAnyByTableName(table string) (bool, error) {
	prefix := strings.TrimSuffix(s.TableName, "crud_log")
	names := []string{table, strings.TrimPrefix(table, prefix)}
	if prefix != "" {
		names = append(names, prefix+strings.TrimPrefix(table, prefix))
	}
	var count int64
	err := s.DB().Model(&model.Log{}).Where("table_name IN ?", names).Count(&count).Error
	return count > 0, err
}

func (s *CrudLogRepository) GetOne(ctx *gin.Context, id int32) (crudLog model.Log, err error) {
	err = s.DBFor(ctx).Model(&model.Log{}).Scopes(s.scoped(ctx)).Where("id=?", id).First(&crudLog).Error
	return
}

func (s *CrudLogRepository) List(ctx *gin.Context) (list []model.Log, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := querybuilder.QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.DBFor(ctx).Model(&model.Log{}).Scopes(s.scoped(ctx)).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *CrudLogRepository) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid crud log ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid crud log id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var list []model.Log
		scoped := tx.Model(&model.Log{}).Scopes(s.scoped(ctx))
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

// 记录CRUD状态
func (s *CrudLogRepository) RecordCrudStatus(ctx *gin.Context, data model.Log) (int32, error) {
	if s.enforcer == nil {
		return 0, data_scope.ErrScopedAccessDenied
	}
	actor, err := s.enforcer.Actor(ctx)
	if err != nil {
		return 0, err
	}
	data.AdminID = actor.AdminID

	if data.ID != 0 {
		result := s.DBFor(ctx).Model(&model.Log{}).Scopes(s.scoped(ctx)).Where("id=?", data.ID).Update("status", data.Status)
		if result.Error != nil {
			return 0, result.Error
		}
		if result.RowsAffected != 1 {
			return 0, gorm.ErrRecordNotFound
		}
		return data.ID, nil
	}
	if err := s.DBFor(ctx).Create(&data).Error; err != nil {
		return 0, err
	}
	return data.ID, nil
}

// RecordCrudError marks a generation as failed and preserves the failing
// stage/message for operators. It intentionally uses the same scoped update
// path as normal status transitions.
func (s *CrudLogRepository) RecordCrudError(ctx *gin.Context, id int32, message string) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	result := s.DBFor(ctx).Model(&model.Log{}).Scopes(s.scoped(ctx)).Where("id=?", id).Updates(map[string]interface{}{
		"status":  "error",
		"comment": message,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateSync applies upload completion markers. Cancellation only clears a
// marker when it still matches the callback's submitted value.
func (s *CrudLogRepository) UpdateSync(ctx *gin.Context, syncIDs map[int32]int, cancelSync bool) error {
	if len(syncIDs) == 0 {
		return nil
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		for id, syncValue := range syncIDs {
			query := tx.Model(&model.Log{}).Scopes(s.scoped(ctx)).Where("id = ?", id)
			value := syncValue
			if cancelSync {
				query = query.Where("sync = ?", syncValue)
				value = 0
			}
			result := query.Update("sync", value)
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	})
}
