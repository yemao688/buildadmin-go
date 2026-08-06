package repository

import (
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	passwordutil "buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/persistence"
	"buildadmin-go/internal/pkg/util"
	"context"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OutUser struct {
	model.User
	Money string `json:"money"`
}

type UserRepository struct {
	persistence.BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
}

func NewUserRepository(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *UserRepository {
	return &UserRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"user", "id", "username,nickname", sqlDB),
		config:    config,
		enforcer:  enforcer,
	}
}

func (s *UserRepository) DealData(ctx context.Context, data *model.User) (*OutUser, error) {
	outUser := OutUser{}
	if err := copier.Copy(&outUser, data); err != nil {
		return nil, err
	}
	outUser.Avatar = util.DefaultUrl(data.Avatar, s.config.App.DefaultAvatar)
	outUser.Money = fmt.Sprintf("%.2f", data.Money)
	return &outUser, nil
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// user.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *UserRepository) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

func (s *UserRepository) GetOne(ctx *gin.Context, id int32) (model.User, error) {
	actor, err := s.enforcer.Actor(ctx)
	if err != nil {
		return model.User{}, err
	}
	return s.GetOneWithActor(ctx, actor, id)
}

// GetOneWithActor loads one scoped user row for an explicit actor.
func (s *UserRepository) GetOneWithActor(ctx context.Context, actor data_scope.Actor, id int32) (model.User, error) {
	data := model.User{}
	err := s.DBFor(ctx).Model(&model.User{}).Scopes(s.scopedWithActor(ctx, actor)).Preload("Admin").Omit("password").Where("`"+s.TableName+"`.id = ?", id).First(&data).Error
	return data, err
}

// scopedWithActor is the transport-free counterpart of scoped for the
// service layer, which receives the actor as an explicit parameter.
func (s *UserRepository) scopedWithActor(ctx context.Context, actor data_scope.Actor) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		enforcer, ok := s.enforcer.(interface {
			ScopeWithActor(context.Context, *gorm.DB, data_scope.Actor, data_scope.OwnerRef) *gorm.DB
		})
		if !ok {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return enforcer.ScopeWithActor(ctx, db, actor, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

func (s *UserRepository) List(ctx *gin.Context) ([]*OutUser, int64, error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	var total int64 = 0
	list := []*model.User{}

	// count 与 find 必须使用独立 statement：复用同一 db 先 Count 再 Find 时，
	// GORM 的 Count 会重置 statement，导致 Find 丢失 scope/搜索 WHERE
	// （用户数据越权可见的根因；与生成器产物 countDB/findDB 模式对齐）。
	countDB := s.DBFor(ctx).Model(&model.User{}).Where(whereS, whereP...)
	countDB = countDB.Scopes(s.scoped(ctx))
	if err = countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	findDB := s.DBFor(ctx).Model(&model.User{}).Preload("Admin").Where(whereS, whereP...)
	findDB = findDB.Scopes(s.scoped(ctx))
	if err := findDB.Omit("password").Order(orderS).Limit(limit).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	result := []*OutUser{}
	for _, v := range list {
		outUser, err := s.DealData(ctx, v)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, outUser)
	}
	return result, total, nil
}

func (s *UserRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	err := s.DBFor(ctx).Where("username=?", username).Take(&model.User{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

// AddWithActor writes a new user owned by the actor (or an explicit owner
// inside the actor's scope). It is the scoped write protocol: hierarchy lock,
// owner validation and the closure self-row check run in one transaction.
func (s *UserRepository) AddWithActor(ctx context.Context, user *model.User, actor data_scope.Actor) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		err := NewAdminHierarchy(s.config).LockHierarchy(ctx, tx)
		if err != nil {
			return err
		}
		ownerID := user.AdminID
		if ownerID == 0 {
			ownerID = actor.AdminID
		}
		if err := s.validateUserOwner(ctx, tx, ownerID, actor); err != nil {
			return err
		}
		user.AdminID = ownerID
		// Creating a user owned by the actor only makes sense if the closure table
		// contains the actor's self-row; otherwise the new row would be invisible.
		if !actor.Unrestricted {
			closureTable := s.config.Database.Prefix + "admin_closure"
			var n int64
			if err := tx.Table(closureTable).Where("ancestor_id = ? AND descendant_id = ?", actor.AdminID, actor.AdminID).Count(&n).Error; err != nil {
				return err
			}
			if n == 0 {
				return cErr.BadRequest("admin scope self-row missing")
			}
		}
		if err := tx.Where("username=?", user.Username).Take(&model.User{}).Error; err == nil {
			return cErr.BadRequest("username already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result := tx.Omit("login_failure", "last_login_time", "last_login_ip").Create(user)
		if result.Error != nil {
			if isDuplicateKeyError(result.Error) {
				return cErr.BadRequest("username already exists")
			}
			return result.Error
		}
		return nil
	})
}

// EditWithActor is the transport-free counterpart of Edit for the service
// layer: the actor is passed explicitly instead of being read from the
// request context.
func (s *UserRepository) EditWithActor(ctx context.Context, user *model.User, password string, actor data_scope.Actor) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	updates := map[string]interface{}{
		"username":  user.Username,
		"nickname":  user.Nickname,
		"email":     user.Email,
		"mobile":    user.Mobile,
		"avatar":    user.Avatar,
		"join_ip":   user.JoinIP,
		"join_time": user.JoinTime,
		"status":    user.Status,
	}
	if password != "" {
		hash, err := passwordutil.Hash(password)
		if err != nil {
			return err
		}
		updates["password"] = hash
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		err := NewAdminHierarchy(s.config).LockHierarchy(ctx, tx)
		if err != nil {
			return err
		}
		if err := tx.Where("id<>? and username=?", user.ID, user.Username).Take(&model.User{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			return cErr.BadRequest("Account not exist")
		}
		var current model.User
		if err := tx.Model(&model.User{}).Scopes(s.scopedWithActor(ctx, actor)).Clauses(clause.Locking{Strength: "UPDATE"}).Where("`"+s.TableName+"`.id = ?", user.ID).First(&current).Error; err != nil {
			return err
		}
		if current.AdminID <= 0 || user.AdminID <= 0 {
			return cErr.BadRequest("invalid administrator owner")
		}
		ownerChanged := current.AdminID != user.AdminID
		if ownerChanged {
			if err := s.validateUserOwner(ctx, tx, user.AdminID, actor); err != nil {
				return err
			}
			if err := s.validateUserLogOwners(tx, current.ID, current.AdminID); err != nil {
				return err
			}
			updates["admin_id"] = user.AdminID
		}
		result := tx.Model(&model.User{}).Scopes(s.scopedWithActor(ctx, actor)).Where("`"+s.TableName+"`.id = ?", user.ID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.User{}).Scopes(s.scopedWithActor(ctx, actor)).Where("`"+s.TableName+"`.id = ?", user.ID).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return gorm.ErrRecordNotFound
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		if ownerChanged {
			if err := s.syncUserLogOwners(tx, current.ID, user.AdminID); err != nil {
				return err
			}
			if err := s.validateUserLogOwners(tx, current.ID, user.AdminID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *UserRepository) validateUserOwner(ctx context.Context, tx *gorm.DB, ownerID int32, actor data_scope.Actor) error {
	if err := data_scope.OwnerInScopeWithActor(ctx, tx, s.enforcer, s.config.Database.Prefix, ownerID, actor); err != nil {
		return err
	}
	var enabled int64
	if err := tx.Table(s.config.Database.Prefix+"admin").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", ownerID, "enable").Count(&enabled).Error; err != nil {
		return err
	}
	if enabled != 1 {
		return cErr.BadRequest("administrator owner is disabled")
	}
	return nil
}

func (s *UserRepository) validateUserLogOwners(tx *gorm.DB, userID, ownerID int32) error {
	for _, table := range []string{s.config.Database.Prefix + "user_money_log"} {
		var logs []struct{ AdminID *int32 }
		if err := tx.Table(table).Clauses(clause.Locking{Strength: "UPDATE"}).Select("admin_id").Where("user_id = ?", userID).Find(&logs).Error; err != nil {
			return err
		}
		for _, log := range logs {
			if log.AdminID == nil || *log.AdminID != ownerID {
				return cErr.BadRequest("user log owner mismatch")
			}
		}
	}
	return nil
}

func (s *UserRepository) syncUserLogOwners(tx *gorm.DB, userID, ownerID int32) error {
	for _, table := range []string{s.config.Database.Prefix + "user_money_log"} {
		if err := tx.Table(table).Where("user_id = ?", userID).Update("admin_id", ownerID).Error; err != nil {
			return err
		}
	}
	return nil
}

// UpdateStatusWithActor updates only the status field for a single user
// within the actor's scope. It is used for switch-style partial edits so that
// the update carries scope and cannot touch out-of-scope rows.
func (s *UserRepository) UpdateStatusWithActor(ctx context.Context, id int32, status string, actor data_scope.Actor) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		result = tx.Model(&model.User{}).Scopes(s.scopedWithActor(ctx, actor)).Where("`"+s.TableName+"`.id = ?", id).Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.User{}).Scopes(s.scopedWithActor(ctx, actor)).Where("`"+s.TableName+"`.id = ?", id).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return gorm.ErrRecordNotFound
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

// LockUsersWithActor loads the users FOR UPDATE inside the actor's scope.
// A missing or out-of-scope id fails with gorm.ErrRecordNotFound, so the
// batch is all-or-nothing. User deletion and balance changes share the same
// user-row lock protocol.
func (s *UserRepository) LockUsersWithActor(ctx context.Context, tx *gorm.DB, ids []int32, actor data_scope.Actor) error {
	var list []model.User
	scoped := tx.Model(&model.User{}).Scopes(s.scopedWithActor(ctx, actor))
	if err := scoped.Clauses(clause.Locking{Strength: "UPDATE"}).Where("`"+s.TableName+"`.id IN ?", ids).Find(&list).Error; err != nil {
		return err
	}
	if len(list) != len(ids) {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CountMoneyLogsByUserIDs counts money-log rows referencing any of the
// given users (the orphan guard data source for the delete flow).
func (s *UserRepository) CountMoneyLogsByUserIDs(tx *gorm.DB, userIDs []int32) (int64, error) {
	var n int64
	if err := tx.Table(s.config.Database.Prefix+"user_money_log").Where("user_id IN ?", userIDs).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// DeleteScopedWithActor deletes the users inside the actor's scope and
// verifies RowsAffected so the batch is all-or-nothing.
func (s *UserRepository) DeleteScopedWithActor(ctx context.Context, tx *gorm.DB, ids []int32, actor data_scope.Actor) error {
	del := tx.Model(&model.User{}).Scopes(s.scopedWithActor(ctx, actor)).Where("`"+s.TableName+"`.id IN ?", ids).Delete(nil)
	if del.Error != nil {
		return del.Error
	}
	if del.RowsAffected != int64(len(ids)) {
		return gorm.ErrRecordNotFound
	}
	return nil
}
