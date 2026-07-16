package model

import (
	"fmt"
	"strings"

	"go-build-admin/app/admin/model/simple"
	"go-build-admin/app/pkg/data_scope"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/safeint"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserMoneyLog 会员余额变动表
type UserMoneyLog struct {
	ID         int32        `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"` // ID
	AdminID    int32        `gorm:"column:admin_id;not null;comment:管理员ID" json:"admin_id"`       // 管理员ID
	UserID     int32        `gorm:"column:user_id;not null;comment:会员ID" json:"user_id"`          // 会员ID
	Money      int32        `gorm:"column:money;not null;comment:变更余额" json:"money"`              // 变更余额
	MoneyCents bool         `gorm:"-" json:"-"`
	Before     int32        `gorm:"column:before;not null;comment:变更前余额" json:"before"`                // 变更前余额
	After      int32        `gorm:"column:after;not null;comment:变更后余额" json:"after"`                  // 变更后余额
	Memo       string       `gorm:"column:memo;not null;comment:备注" json:"memo"`                       // 备注
	CreateTime int64        `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	Admin      simple.Admin `gorm:"foreignKey:AdminID" json:"admin"`
	User       simple.User  `json:"user"`
}

type UserMoneyLogModel struct {
	BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
}

func NewUserMoneyLogModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *UserMoneyLogModel {
	return &UserMoneyLogModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "user_money_log",
			Key:              "id",
			QuickSearchField: "user.username,user.nickname",
			sqlDB:            sqlDB,
		},
		config:   config,
		enforcer: enforcer,
	}
}

func quote(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// user_money_log.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *UserMoneyLogModel) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

func (s *UserMoneyLogModel) userScope(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	userAlias := s.config.Database.Prefix + "user"
	return func(db *gorm.DB) *gorm.DB {
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: userAlias, Column: "admin_id"})
	}
}

func (s *UserMoneyLogModel) userJoin() string {
	userTable := s.config.Database.Prefix + "user"
	return "LEFT JOIN " + quote(userTable) + " AS " + quote("user") + " ON " + quote("user") + ".`id` = " + quote(s.TableName) + ".`user_id`"
}

func (s *UserMoneyLogModel) GetOne(ctx *gin.Context, id int32) (UserMoneyLog, error) {
	data := UserMoneyLog{}
	err := s.sqlDB.Model(&UserMoneyLog{}).Scopes(s.scoped(ctx)).Preload("User").Preload("Admin").Joins(s.userJoin()).Where(quote(s.TableName)+".id = ?", id).First(&data).Error
	return data, err
}

func (s *UserMoneyLogModel) List(ctx *gin.Context) (list []UserMoneyLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.sqlDB.Model(&UserMoneyLog{}).Preload("User").Preload("Admin").Joins(s.userJoin()).Where(whereS, whereP...)
	db = db.Scopes(s.scoped(ctx))
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

// Add creates a balance change log in a single transaction. The target user is
// selected with FOR UPDATE under the actor's scope, the new balance is computed
// with int64 overflow checks and must not become negative, then the user row is
// updated and the log (owned by user.AdminID) is inserted. Any failure rolls
// back both changes.
func (s *UserMoneyLogModel) Add(ctx *gin.Context, userMoneyLog *UserMoneyLog) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	if _, err := s.enforcer.Actor(ctx); err != nil {
		return err
	}

	return s.sqlDB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Model(&User{}).Scopes(s.userScope(ctx)).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userMoneyLog.UserID).Take(&user).Error; err != nil {
			return err
		}
		if user.AdminID == 0 {
			return fmt.Errorf("target user has no owner")
		}

		// HTTP handlers parse yuan exactly into cents. Keep the old model-level
		// integer contract for callers which construct this model directly.
		delta := userMoneyLog.Money
		var err error
		if !userMoneyLog.MoneyCents {
			delta, err = safeint.MulInt32(delta, 100)
			if err != nil {
				return cErr.BadRequest("money amount out of range")
			}
		}
		before := user.Money
		after, err := safeint.AddInt32(before, delta)
		if err != nil {
			return cErr.BadRequest("balance overflow")
		}
		if after < 0 {
			return cErr.BadRequest("insufficient balance")
		}

		res := tx.Model(&User{}).Scopes(s.userScope(ctx)).Where("id = ?", userMoneyLog.UserID).UpdateColumn("money", gorm.Expr("money + ?", delta))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}

		userMoneyLog.AdminID = user.AdminID
		userMoneyLog.UserID = user.ID
		userMoneyLog.Before = before
		userMoneyLog.Money = delta
		userMoneyLog.After = after
		return tx.Create(userMoneyLog).Error
	})
}

func (s *UserMoneyLogModel) Del(ctx *gin.Context, ids interface{}) error {
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
	return s.sqlDB.Transaction(func(tx *gorm.DB) error {
		var list []UserMoneyLog
		scoped := tx.Model(&UserMoneyLog{}).Scopes(s.scoped(ctx))
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
