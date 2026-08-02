package country

import (
	"fmt"

	adminmodel "go-build-admin/internal/admin/model"
	"go-build-admin/internal/conf"
	persistence "go-build-admin/internal/pkg/persistence"

	"go-build-admin/internal/pkg/data_scope"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LanguageContentModel struct {
	persistence.BaseModel
	Policy   data_scope.ResourcePolicy
	Enforcer data_scope.Enforcer
	config   *conf.Configuration
}

func (s *LanguageContentModel) NewRow() any {
	return &LanguageContent{}
}

func NewLanguageContentModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *LanguageContentModel {
	return &LanguageContentModel{
		BaseModel: persistence.NewBaseModel(
			config.Database.Prefix+"country_language_content",
			"id",
			"group,key,id",
			sqlDB,
		),
		Policy: data_scope.ResourcePolicy{
			Mode:           "none",
			OwnerColumn:    "",
			AssignOnCreate: false,
		},
		Enforcer: enforcer,
		config:   config,
	}
}

func (s *LanguageContentModel) scopedDB(ctx *gin.Context) *gorm.DB {
	return s.scopeDB(ctx, s.DBFor(ctx))
}

func (s *LanguageContentModel) scopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
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
func (s *LanguageContentModel) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	return s.scopeDB(ctx, db)
}

func (s *LanguageContentModel) GetOne(ctx *gin.Context, id int64) (languageContent LanguageContent, err error) {
	db := s.scopedDB(ctx).Session(&gorm.Session{})
	db.Statement.Table = s.TableName
	err = db.Where("id=?", id).First(&languageContent).Error
	return
}

func (s *LanguageContentModel) List(ctx *gin.Context) (list []LanguageContent, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := adminmodel.QueryBuilder(ctx, s.TableInfo(), nil)
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

func (s *LanguageContentModel) Add(ctx *gin.Context, languageContent LanguageContent) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {

		if err := tx.Table(s.TableName).Create(&languageContent).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *LanguageContentModel) Edit(ctx *gin.Context, languageContent LanguageContent) error {
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

		res := tx.Table(s.TableName).Model(&languageContent).Where("id = ?", languageContent.ID).Select("lan", "group", "key", "type", "value").Updates(&languageContent)
		if err := res.Error; err != nil {
			return err
		}
		switch res.RowsAffected {
		case 1:
			return nil
		case 0:
			var visible int64
			if err := tx.Table(s.TableName).Model(&LanguageContent{}).Where("id = ?", languageContent.ID).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return gorm.ErrRecordNotFound
		default:
			return fmt.Errorf("unexpected edit rows affected: %d", res.RowsAffected)
		}
	})
}

func (s *LanguageContentModel) Del(ctx *gin.Context, ids interface{}) error {
	normalizedIDs, err := normalizeLanguageContentIDs(ids)
	if err != nil {
		return err
	}
	if len(normalizedIDs) == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		tx = s.scopeDB(ctx, tx)

		var visible int64
		if err := tx.Table(s.TableName).Model(&LanguageContent{}).Where("id IN ?", normalizedIDs).Count(&visible).Error; err != nil {
			return err
		}
		if visible != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}

		res := tx.Table(s.TableName).Where("id IN ?", normalizedIDs).Delete(&LanguageContent{})
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func normalizeLanguageContentIDs(ids interface{}) ([]int64, error) {
	raw, ok := ids.([]int64)
	if !ok {
		return nil, fmt.Errorf("invalid id ids type %T", ids)
	}
	seen := make(map[int64]struct{}, len(raw))
	result := make([]int64, 0, len(raw))
	for _, id := range raw {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result, nil
}
