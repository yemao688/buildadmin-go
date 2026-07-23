package model

import (
	"fmt"
	"time"

	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Test 测试表
type Test struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // ID
	Editor     string `gorm:"column:editor;comment:富文本" json:"editor"`                             // 富文本
	Status     int32  `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	Weigh      int32  `gorm:"column:weigh;comment:权重" json:"weigh"`                                // 权重
	UpdateTime int64  `gorm:"column:update_time;comment:修改时间" json:"update_time"`                  // 修改时间
	CreateTime int64  `gorm:"column:create_time;comment:创建时间" json:"create_time"`                  // 创建时间
}

type TestModel struct {
	BaseModel
	Policy   data_scope.ResourcePolicy
	Enforcer data_scope.Enforcer
}

func NewTestModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *TestModel {
	return &TestModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "test",
			Key:              "id",
			QuickSearchField: "id",
			sqlDB:            sqlDB,
		},
		Policy: data_scope.ResourcePolicy{
			Mode:           "none",
			OwnerColumn:    "",
			AssignOnCreate: false,
		},
		Enforcer: enforcer,
	}
}

func (s *TestModel) scopedDB(ctx *gin.Context) *gorm.DB {
	return s.scopeDB(ctx, s.DBFor(ctx))
}

func (s *TestModel) scopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	if s.Policy.Mode == data_scope.ModeNone {
		return db
	}
	if s.Enforcer == nil {
		tx := db.Session(&gorm.Session{})
		_ = tx.AddError(data_scope.ErrScopedAccessDenied)
		return tx
	}
	return s.Enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: s.Policy.OwnerColumn})
}

// ScopeDB exposes the generated model's data-scope application to generic CRUD handlers.
func (s *TestModel) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	return s.scopeDB(ctx, db)
}

func (s *TestModel) GetOne(ctx *gin.Context, id int32) (test Test, err error) {
	db := s.scopedDB(ctx).Session(&gorm.Session{})
	db.Statement.Table = s.TableName
	err = db.Where("id=?", id).First(&test).Error
	return
}

func (s *TestModel) List(ctx *gin.Context) (list []Test, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	countDB := s.scopedDB(ctx).Session(&gorm.Session{})
	countDB.Statement.Table = s.TableName
	countDB = countDB.Where(whereS, whereP...)
	if err = countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	findDB := s.scopedDB(ctx).Session(&gorm.Session{})
	findDB.Statement.Table = s.TableName
	findDB = findDB.Where(whereS, whereP...)
	err = findDB.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *TestModel) Add(ctx *gin.Context, test Test) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		test.CreateTime = time.Now().Unix()
		test.UpdateTime = time.Now().Unix()

		if err := tx.Table(s.TableName).Create(&test).Error; err != nil {
			return err
		}
		if test.Weigh == 0 {
			if err := tx.Table(s.TableName).Where("id = ?", test.ID).Update("weigh", test.ID).Error; err != nil {
				return err
			}
			test.Weigh = int32(test.ID)
		}
		return nil
	})
}

func (s *TestModel) Edit(ctx *gin.Context, test Test) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		tx = s.scopeDB(ctx, tx)
		test.UpdateTime = time.Now().Unix()

		res := tx.Table(s.TableName).Model(&test).Where("id = ?", test.ID).Select("status", "weigh", "update_time").Updates(&test)
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *TestModel) Del(ctx *gin.Context, ids interface{}) error {
	normalizedIDs, err := normalizeTestIDs(ids)
	if err != nil {
		return err
	}
	if len(normalizedIDs) == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		tx = s.scopeDB(ctx, tx)

		var visible int64
		if err := tx.Table(s.TableName).Model(&Test{}).Where("id IN ?", normalizedIDs).Count(&visible).Error; err != nil {
			return err
		}
		if visible != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}

		res := tx.Table(s.TableName).Where("id IN ?", normalizedIDs).Delete(&Test{})
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func normalizeTestIDs(ids interface{}) ([]int32, error) {
	raw, ok := ids.([]int32)
	if !ok {
		return nil, fmt.Errorf("invalid id ids type %T", ids)
	}
	seen := make(map[int32]struct{}, len(raw))
	result := make([]int32, 0, len(raw))
	for _, id := range raw {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}
