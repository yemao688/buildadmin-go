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

type CountryLanguageRepository struct {
	persistence.BaseModel
	Policy   data_scope.ResourcePolicy
	Enforcer data_scope.Enforcer
	config   *conf.Configuration
}

func (s *CountryLanguageRepository) NewRow() any {
	return &model.CountryLanguage{}
}

func NewCountryLanguageRepository(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *CountryLanguageRepository {
	return &CountryLanguageRepository{
		BaseModel: persistence.NewBaseModel(
			config.Database.Prefix+"country_language",
			"id",
			"lan,name,id",
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

func (s *CountryLanguageRepository) scopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
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
func (s *CountryLanguageRepository) readScopedDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
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
func (s *CountryLanguageRepository) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	return s.scopeDB(ctx, db)
}

func (s *CountryLanguageRepository) GetOne(ctx *gin.Context, id int64) (countryLanguage model.CountryLanguage, err error) {
	db := s.readScopedDB(ctx, s.DBFor(ctx)).Session(&gorm.Session{})
	db.Statement.Table = s.TableName
	err = db.Where("id=?", id).First(&countryLanguage).Error
	return
}

func (s *CountryLanguageRepository) List(ctx *gin.Context) (list []model.CountryLanguage, total int64, err error) {
	tableInfo := s.TableInfo()
	tableInfo.FieldTypes = map[string]string{"id": "bigint", "lan": "varchar", "name": "varchar", "remark": "varchar", "status": "tinyint", "weigh": "int"}
	tableInfo.DefaultOrder = "weigh,desc"
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, tableInfo, nil)
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

func (s *CountryLanguageRepository) Add(ctx *gin.Context, countryLanguage model.CountryLanguage) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {

		if err := tx.Table(s.TableName).Create(&countryLanguage).Error; err != nil {
			return err
		}
		if countryLanguage.Weigh == 0 {
			if err := tx.Table(s.TableName).Where("id = ?", countryLanguage.ID).Update("weigh", countryLanguage.ID).Error; err != nil {
				return err
			}
			countryLanguage.Weigh = int32(countryLanguage.ID)
		}
		return nil
	})
}

func (s *CountryLanguageRepository) Edit(ctx *gin.Context, countryLanguage model.CountryLanguage) error {
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

		res := tx.Table(s.TableName).Model(&countryLanguage).Where("id = ?", countryLanguage.ID).Select("lan", "name", "remark", "status", "weigh").Updates(&countryLanguage)
		if err := res.Error; err != nil {
			return err
		}
		switch res.RowsAffected {
		case 1:
			return nil
		case 0:
			var visible int64
			if err := tx.Table(s.TableName).Model(&model.CountryLanguage{}).Where("id = ?", countryLanguage.ID).Count(&visible).Error; err != nil {
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

func (s *CountryLanguageRepository) Del(ctx *gin.Context, ids interface{}) error {
	normalizedIDs, err := normalizeCountryLanguageIDs(ids)
	if err != nil {
		return err
	}
	if len(normalizedIDs) == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		tx = s.scopeDB(ctx, tx)

		var visible int64
		if err := tx.Table(s.TableName).Model(&model.CountryLanguage{}).Where("id IN ?", normalizedIDs).Count(&visible).Error; err != nil {
			return err
		}
		if visible != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}

		res := tx.Table(s.TableName).Where("id IN ?", normalizedIDs).Delete(&model.CountryLanguage{})
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func normalizeCountryLanguageIDs(ids interface{}) ([]int64, error) {
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
