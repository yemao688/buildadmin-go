package user

import (
	"fmt"
	"strings"

	"go-build-admin/internal/admin/model/simple"
	"go-build-admin/internal/pkg/data_scope"
	cErr "go-build-admin/internal/pkg/error"
	"go-build-admin/internal/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// TableName 经全局命名策略解析（前缀安全），对齐真实表 user_money_log。
func (MoneyLog) TableName(namer schema.Namer) string {
	return namer.TableName("user_money_log")
}

// MoneyLog 会员余额变动表
type MoneyLog struct {
	ID         int32        `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`      // ID
	AdminID    int32        `gorm:"column:admin_id;not null;comment:管理员ID" json:"admin_id"`            // 管理员ID
	UserID     int32        `gorm:"column:user_id;not null;comment:会员ID" json:"user_id"`               // 会员ID
	Money      float64      `gorm:"column:money;not null;comment:变更余额" json:"money"`                   // 变更余额
	Before     float64      `gorm:"column:before;not null;comment:变更前余额" json:"before"`                // 变更前余额
	After      float64      `gorm:"column:after;not null;comment:变更后余额" json:"after"`                  // 变更后余额
	Memo       string       `gorm:"column:memo;not null;comment:备注" json:"memo"`                       // 备注
	CreateTime int64        `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"` // 创建时间
	Admin      simple.Admin `gorm:"foreignKey:AdminID" json:"admin"`
	User       simple.User  `json:"user"`
}

type MoneyLogModel struct {
	BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
}

// ApplyMoneyDeltaInput contains the transport-neutral inputs for a balance
// change. Log.AdminID is derived from the target user's owner; it is not the
// operator identity. OperatorAdminID is zero for system flows and is kept
// separate so callers do not confuse those two identities.
type ApplyMoneyDeltaInput struct {
	UserID          int32
	Delta           float64
	Log             *MoneyLog
	OperatorAdminID int32
	Scope           func(db *gorm.DB) *gorm.DB
}

func NewMoneyLogModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *MoneyLogModel {
	return &MoneyLogModel{
		BaseModel: NewBaseModel(config.Database.Prefix+"user_money_log", "id", "user.username,user.nickname", sqlDB),
		config:    config,
		enforcer:  enforcer,
	}
}

func quote(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// user_money_log.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *MoneyLogModel) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

func (s *MoneyLogModel) userScope(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	userAlias := s.config.Database.Prefix + "user"
	return func(db *gorm.DB) *gorm.DB {
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: userAlias, Column: "admin_id"})
	}
}

func (s *MoneyLogModel) userJoin() string {
	userTable := s.config.Database.Prefix + "user"
	return "LEFT JOIN " + quote(userTable) + " AS " + quote("user") + " ON " + quote("user") + ".`id` = " + quote(s.TableName) + ".`user_id`"
}

func (s *MoneyLogModel) GetOne(ctx *gin.Context, id int32) (MoneyLog, error) {
	data := MoneyLog{}
	err := s.DB().Model(&MoneyLog{}).Scopes(s.scoped(ctx)).Preload("User").Preload("Admin").Joins(s.userJoin()).Where(quote(s.TableName)+".id = ?", id).First(&data).Error
	return data, err
}

func (s *MoneyLogModel) List(ctx *gin.Context) (list []MoneyLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.DB().Model(&MoneyLog{}).Preload("User").Preload("Admin").Joins(s.userJoin()).Where(whereS, whereP...)
	db = db.Scopes(s.scoped(ctx))
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

// ApplyMoneyDelta applies one balance change using the transaction supplied
// by the caller. A nil Scope is an unrestricted system flow. The caller must
// provide a transaction when the row lock and balance/log write need to be
// atomic.
func (s *MoneyLogModel) ApplyMoneyDelta(tx *gorm.DB, input ApplyMoneyDeltaInput) error {
	if tx == nil {
		return gorm.ErrInvalidDB
	}
	if input.Log == nil {
		return fmt.Errorf("money log is nil")
	}

	applyScope := func(db *gorm.DB) *gorm.DB {
		if input.Scope == nil {
			return db
		}
		return input.Scope(db)
	}

	var user User
	if err := applyScope(tx.Model(&User{})).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", input.UserID).Take(&user).Error; err != nil {
		return err
	}
	if user.AdminID == 0 {
		return fmt.Errorf("target user has no owner")
	}

	before := user.Money
	after := before + input.Delta
	if after < 0 {
		return cErr.BadRequest("insufficient balance")
	}

	res := applyScope(tx.Model(&User{})).Where("id = ?", input.UserID).UpdateColumn("money", gorm.Expr("money + ?", input.Delta))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}

	input.Log.AdminID = user.AdminID
	input.Log.UserID = user.ID
	input.Log.Before = before
	input.Log.Money = input.Delta
	input.Log.After = after
	return tx.Create(input.Log).Error
}

// Add creates a balance change log in a single transaction. The target user is
// selected with FOR UPDATE under the actor's scope, the new balance is computed
// and must not become negative, then the user row is
// updated and the log (owned by user.AdminID) is inserted. Any failure rolls
// back both changes.
func (s *MoneyLogModel) Add(ctx *gin.Context, userMoneyLog *MoneyLog) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	actor, err := s.enforcer.Actor(ctx)
	if err != nil {
		return err
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		return s.ApplyMoneyDelta(tx, ApplyMoneyDeltaInput{
			UserID:          userMoneyLog.UserID,
			Delta:           userMoneyLog.Money,
			Log:             userMoneyLog,
			OperatorAdminID: actor.AdminID,
			Scope:           s.userScope(ctx),
		})
	})
}

func (s *MoneyLogModel) Del(ctx *gin.Context, ids interface{}) error {
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
		var list []MoneyLog
		scoped := tx.Model(&MoneyLog{}).Scopes(s.scoped(ctx))
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
