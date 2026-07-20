package model

import (
	"fmt"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CountryLanguageContent 全局语言内容
type CountryLanguageContent struct {
	ID    int64  `gorm:"column:id;primaryKey;autoIncrement:true;comment:主键" json:"id"` // 主键
	Lan   string `gorm:"column:lan;not null;comment:语言代码" json:"lan"`                  // 语言代码
	Group string `gorm:"column:group;not null;comment:分组" json:"group"`                // 分组
	Key   string `gorm:"column:key;not null;comment:键" json:"key"`                     // 键
	Type  int32  `gorm:"column:type;not null;comment:类型:0=文本,1=富文本,2=图片" json:"type"`  // 类型:0=文本,1=富文本,2=图片
	Value string `gorm:"column:value;comment:值" json:"value"`                          // 值
}

type CountryLanguageContentModel struct {
	BaseModel
	Policy   data_scope.ResourcePolicy
	Enforcer data_scope.Enforcer
}

func NewCountryLanguageContentModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *CountryLanguageContentModel {
	return &CountryLanguageContentModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "country_language_content",
			Key:              "id",
			QuickSearchField: "group,key,id",
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

func (s *CountryLanguageContentModel) scopedDB(ctx *gin.Context) *gorm.DB {
	return s.scopeDB(ctx, s.DBFor(ctx))
}

func (s *CountryLanguageContentModel) scopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
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
func (s *CountryLanguageContentModel) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	return s.scopeDB(ctx, db)
}

func (s *CountryLanguageContentModel) GetOne(ctx *gin.Context, id int64) (countryLanguageContent CountryLanguageContent, err error) {
	db := s.scopedDB(ctx).Session(&gorm.Session{})
	db.Statement.Table = s.TableName
	err = db.Where("id=?", id).First(&countryLanguageContent).Error
	return
}

func (s *CountryLanguageContentModel) List(ctx *gin.Context) (list []CountryLanguageContent, total int64, err error) {
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

func (s *CountryLanguageContentModel) Add(ctx *gin.Context, countryLanguageContent CountryLanguageContent) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		return tx.Table(s.TableName).Create(&countryLanguageContent).Error
	})
}

func (s *CountryLanguageContentModel) Edit(ctx *gin.Context, countryLanguageContent CountryLanguageContent) error {
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
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *CountryLanguageContentModel) Del(ctx *gin.Context, ids interface{}) error {
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
		if err := tx.Table(s.TableName).Model(&CountryLanguageContent{}).Where("id IN ?", normalizedIDs).Count(&visible).Error; err != nil {
			return err
		}
		if visible != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}

		res := tx.Table(s.TableName).Where("id IN ?", normalizedIDs).Delete(&CountryLanguageContent{})
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
