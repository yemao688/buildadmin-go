package repository

import (
	"errors"
	"fmt"
	"strings"

	"buildadmin-go/internal/common/money"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/persistence"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MoneyLogRepository struct {
	persistence.BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
	balance  *money.BalanceService
}

func NewMoneyLogRepository(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer, balance *money.BalanceService) *MoneyLogRepository {
	return &MoneyLogRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"user_money_log", "id", "user.username,user.nickname", sqlDB),
		config:    config,
		enforcer:  enforcer,
		balance:   balance,
	}
}

func quote(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// user_money_log.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *MoneyLogRepository) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

func (s *MoneyLogRepository) userScope(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	userAlias := s.config.Database.Prefix + "user"
	return func(db *gorm.DB) *gorm.DB {
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: userAlias, Column: "admin_id"})
	}
}

func (s *MoneyLogRepository) userJoin() string {
	userTable := s.config.Database.Prefix + "user"
	return "LEFT JOIN " + quote(userTable) + " AS " + quote("user") + " ON " + quote("user") + ".`id` = " + quote(s.TableName) + ".`user_id`"
}

func (s *MoneyLogRepository) GetOne(ctx *gin.Context, id int32) (model.MoneyLog, error) {
	data := model.MoneyLog{}
	err := s.DB().Model(&model.MoneyLog{}).Scopes(s.scoped(ctx)).Preload("User").Preload("Admin").Joins(s.userJoin()).Where(quote(s.TableName)+".id = ?", id).First(&data).Error
	return data, err
}

func (s *MoneyLogRepository) List(ctx *gin.Context) (list []model.MoneyLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.DB().Model(&model.MoneyLog{}).Preload("User").Preload("Admin").Joins(s.userJoin()).Where(whereS, whereP...)
	db = db.Scopes(s.scoped(ctx))
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

// Add creates a balance change log in a single transaction via the shared
// money domain service: the target user is selected with FOR UPDATE under the
// actor's scope, the new balance is computed and must not become negative,
// then the user row is updated and the log (owned by user.AdminID) is
// inserted. Any failure rolls back both changes.
func (s *MoneyLogRepository) Add(ctx *gin.Context, userMoneyLog *model.MoneyLog) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	if _, err := s.enforcer.Actor(ctx); err != nil {
		return err
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		created, err := s.balance.ApplyDelta(tx, money.ApplyInput{
			UserID: userMoneyLog.UserID,
			Delta:  userMoneyLog.Money,
			Log:    userMoneyLog,
			Scope:  s.userScope(ctx),
		})
		if err != nil {
			if errors.Is(err, money.ErrInsufficientBalance) {
				return cErr.BadRequest("insufficient balance")
			}
			return err
		}
		*userMoneyLog = *created
		return nil
	})
}

func (s *MoneyLogRepository) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid user money log ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid user money log id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var list []model.MoneyLog
		scoped := tx.Model(&model.MoneyLog{}).Scopes(s.scoped(ctx))
		if err := scoped.Where(quote(s.TableName)+".id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		del := scoped.Where(quote(s.TableName)+".id IN ?", normalized).Delete(nil)
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
