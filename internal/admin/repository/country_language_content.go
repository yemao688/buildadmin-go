package repository

import (
	"fmt"

	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	persistence "buildadmin-go/internal/pkg/persistence"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CountryLanguageContentRepository struct {
	persistence.BaseModel
	Policy   data_scope.ResourcePolicy
	Enforcer data_scope.Enforcer
	config   *conf.Configuration
}

func (s *CountryLanguageContentRepository) NewRow() any {
	return &model.CountryLanguageContent{}
}

func NewCountryLanguageContentRepository(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *CountryLanguageContentRepository {
	return &CountryLanguageContentRepository{
		BaseModel: persistence.NewBaseModel(
			config.Database.Prefix+"country_language_content",
			"id",
			"group,key,id",
			sqlDB,
		),
		Policy: data_scope.ResourcePolicy{
			Mode:            "none",
			OwnerColumn:     "",
			ReadExtraOwners: []string{},
			AssignOnCreate:  false,
		},
		Enforcer: enforcer,
		config:   config,
	}
}

func (s *CountryLanguageContentRepository) scopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
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

// readScopedDB applies the primary and optional extra owner read scope.
func (s *CountryLanguageContentRepository) readScopedDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	if s.Policy.Mode == data_scope.ModeNone {
		return db
	}
	if s.Enforcer == nil {
		tx := db.Session(&gorm.Session{})
		_ = tx.AddError(data_scope.ErrScopedAccessDenied)
		return tx
	}
	extras := make([]data_scope.OwnerRef, 0, len(s.Policy.ReadExtraOwners))
	for _, column := range s.Policy.ReadExtraOwners {
		extras = append(extras, data_scope.OwnerRef{TableAlias: s.TableName, Column: column})
	}
	return data_scope.ScopeRead(ctx, db, s.Enforcer, data_scope.OwnerRef{TableAlias: s.TableName, Column: s.Policy.OwnerColumn}, extras)
}

// ScopeDB exposes the generated repository's data-scope application to generic CRUD handlers.
func (s *CountryLanguageContentRepository) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	return s.scopeDB(ctx, db)
}

func (s *CountryLanguageContentRepository) GetOne(ctx *gin.Context, id int64) (countryLanguageContent model.CountryLanguageContent, err error) {
	db := s.readScopedDB(ctx, s.DBFor(ctx)).Session(&gorm.Session{})
	db.Statement.Table = s.TableName
	err = db.Where("id=?", id).First(&countryLanguageContent).Error
	return
}

func (s *CountryLanguageContentRepository) List(ctx *gin.Context) (list []model.CountryLanguageContent, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	countDB := s.readScopedDB(ctx, s.DBFor(ctx)).Session(&gorm.Session{})
	countDB.Statement.Table = s.TableName
	countDB = countDB.Where(whereS, whereP...)
	if err = countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	findDB := s.readScopedDB(ctx, s.DBFor(ctx)).Session(&gorm.Session{})
	findDB.Statement.Table = s.TableName
	findDB = findDB.Where(whereS, whereP...)
	err = findDB.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *CountryLanguageContentRepository) Add(ctx *gin.Context, countryLanguageContent model.CountryLanguageContent) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {

		if err := tx.Table(s.TableName).Create(&countryLanguageContent).Error; err != nil {
			return err
		}
		return nil
	})
}

func (s *CountryLanguageContentRepository) Edit(ctx *gin.Context, countryLanguageContent model.CountryLanguageContent) error {
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

		res := tx.Table(s.TableName).Model(&countryLanguageContent).Where("id = ?", countryLanguageContent.ID).Select("lan", "group", "key", "type", "value").Updates(&countryLanguageContent)
		if err := res.Error; err != nil {
			return err
		}
		switch res.RowsAffected {
		case 1:
			return nil
		case 0:
			var visible int64
			if err := tx.Table(s.TableName).Model(&model.CountryLanguageContent{}).Where("id = ?", countryLanguageContent.ID).Count(&visible).Error; err != nil {
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

func (s *CountryLanguageContentRepository) Del(ctx *gin.Context, ids interface{}) error {
	normalizedIDs, err := normalizeCountryLanguageContentIDs(ids)
	if err != nil {
		return err
	}
	if len(normalizedIDs) == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		tx = s.scopeDB(ctx, tx)

		var visible int64
		if err := tx.Table(s.TableName).Model(&model.CountryLanguageContent{}).Where("id IN ?", normalizedIDs).Count(&visible).Error; err != nil {
			return err
		}
		if visible != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}

		res := tx.Table(s.TableName).Where("id IN ?", normalizedIDs).Delete(&model.CountryLanguageContent{})
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func normalizeCountryLanguageContentIDs(ids interface{}) ([]int64, error) {
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
