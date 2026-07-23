package model

import (
	"fmt"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CountryCurrency 全局货币
type CountryCurrency struct {
	ID     int64   `gorm:"column:id;primaryKey;autoIncrement:true;comment:主键" json:"id"`        // 主键
	Code   string  `gorm:"column:code;not null;comment:货币代码" json:"code"`                       // 货币代码
	Name   string  `gorm:"column:name;not null;comment:货币名称" json:"name"`                       // 货币名称
	Symbol string  `gorm:"column:symbol;not null;comment:货币符号" json:"symbol"`                   // 货币符号
	Rate   float64 `gorm:"column:rate;not null;default:1.00000000;comment:汇率" json:"rate"`      // 汇率
	Status int32   `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	Weigh  int32   `gorm:"column:weigh;not null;comment:权重" json:"weigh"`                       // 权重
}

type CountryCurrencyModel struct {
	BaseModel
	Policy   data_scope.ResourcePolicy
	Enforcer data_scope.Enforcer
}

func NewCountryCurrencyModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *CountryCurrencyModel {
	return &CountryCurrencyModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "country_currency",
			Key:              "id",
			QuickSearchField: "code,name,id",
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

func (s *CountryCurrencyModel) scopedDB(ctx *gin.Context) *gorm.DB {
	return s.scopeDB(ctx, s.DBFor(ctx))
}

func (s *CountryCurrencyModel) scopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
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
func (s *CountryCurrencyModel) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	return s.scopeDB(ctx, db)
}

func (s *CountryCurrencyModel) GetOne(ctx *gin.Context, id int64) (countryCurrency CountryCurrency, err error) {
	db := s.scopedDB(ctx).Session(&gorm.Session{})
	db.Statement.Table = s.TableName
	err = db.Where("id=?", id).First(&countryCurrency).Error
	return
}

func (s *CountryCurrencyModel) List(ctx *gin.Context) (list []CountryCurrency, total int64, err error) {
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

func (s *CountryCurrencyModel) Add(ctx *gin.Context, countryCurrency CountryCurrency) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Table(s.TableName).Create(&countryCurrency).Error; err != nil {
			return err
		}
		if countryCurrency.Weigh == 0 {
			if err := tx.Table(s.TableName).Where("id = ?", countryCurrency.ID).Update("weigh", countryCurrency.ID).Error; err != nil {
				return err
			}
			countryCurrency.Weigh = int32(countryCurrency.ID)
		}
		return nil
	})
}

func (s *CountryCurrencyModel) Edit(ctx *gin.Context, countryCurrency CountryCurrency) error {
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

		res := tx.Table(s.TableName).Model(&countryCurrency).Where("id = ?", countryCurrency.ID).Select("code", "name", "symbol", "rate", "status", "weigh").Updates(&countryCurrency)
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *CountryCurrencyModel) Del(ctx *gin.Context, ids interface{}) error {
	normalizedIDs, err := normalizeCountryCurrencyIDs(ids)
	if err != nil {
		return err
	}
	if len(normalizedIDs) == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		tx = s.scopeDB(ctx, tx)

		var visible int64
		if err := tx.Table(s.TableName).Model(&CountryCurrency{}).Where("id IN ?", normalizedIDs).Count(&visible).Error; err != nil {
			return err
		}
		if visible != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}

		res := tx.Table(s.TableName).Where("id IN ?", normalizedIDs).Delete(&CountryCurrency{})
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func normalizeCountryCurrencyIDs(ids interface{}) ([]int64, error) {
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
