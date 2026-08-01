package country

import (
	"fmt"

	adminmodel "go-build-admin/app/admin/model"
	"go-build-admin/conf"

	"go-build-admin/app/pkg/data_scope"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Language 全局语言
type Language struct {
	ID     int64  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`        // 主键
	Lan    string `gorm:"column:lan;not null;comment:语言代码" json:"lan"`                         // 语言代码
	Name   string `gorm:"column:name;not null;comment:语言名称" json:"name"`                       // 语言名称
	Remark string `gorm:"column:remark;not null;comment:备注" json:"remark"`                     // 备注
	Status int32  `gorm:"column:status;not null;default:1;comment:状态:0=禁用,1=启用" json:"status"` // 状态:0=禁用,1=启用
	Weigh  int32  `gorm:"column:weigh;not null;comment:权重" json:"weigh"`                       // 权重
}

type LanguageModel struct {
	adminmodel.BaseModel
	Policy   data_scope.ResourcePolicy
	Enforcer data_scope.Enforcer
	config   *conf.Configuration
}

func (s *LanguageModel) NewRow() any {
	return &Language{}
}

func NewLanguageModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *LanguageModel {
	return &LanguageModel{
		BaseModel: adminmodel.NewBaseModel(
			config.Database.Prefix+"country_language",
			"id",
			"lan,name,id",
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

func (s *LanguageModel) scopedDB(ctx *gin.Context) *gorm.DB {
	return s.scopeDB(ctx, s.DBFor(ctx))
}

func (s *LanguageModel) scopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
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
func (s *LanguageModel) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	return s.scopeDB(ctx, db)
}

func (s *LanguageModel) GetOne(ctx *gin.Context, id int64) (language Language, err error) {
	db := s.scopedDB(ctx).Session(&gorm.Session{})
	db.Statement.Table = s.TableName
	err = db.Where("id=?", id).First(&language).Error
	return
}

func (s *LanguageModel) List(ctx *gin.Context) (list []Language, total int64, err error) {
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

func (s *LanguageModel) Add(ctx *gin.Context, language Language) error {
	if s.Policy.Mode != data_scope.ModeNone {
		if s.Enforcer == nil {
			return data_scope.ErrScopedAccessDenied
		}
		if _, err := s.Enforcer.Actor(ctx); err != nil {
			return err
		}
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {

		if err := tx.Table(s.TableName).Create(&language).Error; err != nil {
			return err
		}
		if language.Weigh == 0 {
			if err := tx.Table(s.TableName).Where("id = ?", language.ID).Update("weigh", language.ID).Error; err != nil {
				return err
			}
			language.Weigh = int32(language.ID)
		}
		return nil
	})
}

func (s *LanguageModel) Edit(ctx *gin.Context, language Language) error {
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

		res := tx.Table(s.TableName).Model(&language).Where("id = ?", language.ID).Select("lan", "name", "remark", "status", "weigh").Updates(&language)
		if err := res.Error; err != nil {
			return err
		}
		switch res.RowsAffected {
		case 1:
			return nil
		case 0:
			var visible int64
			if err := tx.Table(s.TableName).Model(&Language{}).Where("id = ?", language.ID).Count(&visible).Error; err != nil {
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

func (s *LanguageModel) Del(ctx *gin.Context, ids interface{}) error {
	normalizedIDs, err := normalizeLanguageIDs(ids)
	if err != nil {
		return err
	}
	if len(normalizedIDs) == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		tx = s.scopeDB(ctx, tx)

		var visible int64
		if err := tx.Table(s.TableName).Model(&Language{}).Where("id IN ?", normalizedIDs).Count(&visible).Error; err != nil {
			return err
		}
		if visible != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}

		res := tx.Table(s.TableName).Where("id IN ?", normalizedIDs).Delete(&Language{})
		if err := res.Error; err != nil {
			return err
		}
		if res.RowsAffected != int64(len(normalizedIDs)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func normalizeLanguageIDs(ids interface{}) ([]int64, error) {
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
