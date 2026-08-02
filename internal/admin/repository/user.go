package repository

import (
	"errors"
	"fmt"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	passwordutil "buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/persistence"
	"buildadmin-go/internal/utils"

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

func (s *UserRepository) DealData(ctx *gin.Context, data *model.User) (*OutUser, error) {
	outUser := OutUser{}
	if err := copier.Copy(&outUser, data); err != nil {
		return nil, err
	}
	outUser.Avatar = utils.DefaultUrl(data.Avatar, s.config.App.DefaultAvatar)
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
	data := model.User{}
	err := s.DBFor(ctx).Model(&model.User{}).Scopes(s.scoped(ctx)).Preload("Admin").Omit("password").Where("`"+s.TableName+"`.id = ?", id).First(&data).Error
	return data, err
}

func (s *UserRepository) List(ctx *gin.Context) ([]*OutUser, int64, error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	var total int64 = 0
	list := []*model.User{}

	db := s.DBFor(ctx).Model(&model.User{}).Where(whereS, whereP...)
	db = db.Preload("Admin")
	db = db.Scopes(s.scoped(ctx))
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Omit("password").Order(orderS).Limit(limit).Offset(offset).Find(&list).Error; err != nil {
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

func (s *UserRepository) Add(ctx *gin.Context, user *model.User) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	actor, err := s.enforcer.Actor(ctx)
	if err != nil {
		return err
	}

	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if ctx == nil || ctx.Request == nil {
			return data_scope.ErrScopedAccessDenied
		}
		err := NewAdminHierarchy(s.config).LockHierarchy(ctx.Request.Context(), tx)
		if err != nil {
			return err
		}
		ownerID := user.AdminID
		if ownerID == 0 {
			ownerID = actor.AdminID
		}
		if err := s.validateUserOwner(ctx, tx, ownerID); err != nil {
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


func (s *UserRepository) UsernameExists(ctx *gin.Context, username string) (bool, error) {
	err := s.DBFor(ctx).Where("username=?", username).Take(&model.User{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (s *UserRepository) Edit(ctx *gin.Context, user *model.User, password string) error {
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
		if ctx == nil || ctx.Request == nil {
			return data_scope.ErrScopedAccessDenied
		}
		err := NewAdminHierarchy(s.config).LockHierarchy(ctx.Request.Context(), tx)
		if err != nil {
			return err
		}
		if err := tx.Where("id<>? and username=?", user.ID, user.Username).Take(&model.User{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			return cErr.BadRequest("Account not exist")
		}
		var current model.User
		if err := tx.Model(&model.User{}).Scopes(s.scoped(ctx)).Clauses(clause.Locking{Strength: "UPDATE"}).Where("`"+s.TableName+"`.id = ?", user.ID).First(&current).Error; err != nil {
			return err
		}
		if current.AdminID <= 0 || user.AdminID <= 0 {
			return cErr.BadRequest("invalid administrator owner")
		}
		ownerChanged := current.AdminID != user.AdminID
		if ownerChanged {
			if err := s.validateUserOwner(ctx, tx, user.AdminID); err != nil {
				return err
			}
			if err := s.validateUserLogOwners(tx, current.ID, current.AdminID); err != nil {
				return err
			}
			updates["admin_id"] = user.AdminID
		}
		result := tx.Model(&model.User{}).Scopes(s.scoped(ctx)).Where("`"+s.TableName+"`.id = ?", user.ID).Updates(updates)
		if result.Error != nil {
			return result.Error
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

func (s *UserRepository) validateUserOwner(ctx *gin.Context, tx *gorm.DB, ownerID int32) error {
	if err := data_scope.OwnerInScope(ctx, tx, s.enforcer, s.config.Database.Prefix, ownerID); err != nil {
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

func (s *UserRepository) ResetPassword(ctx *gin.Context, id int32, password string) error {
	hash, err := passwordutil.Hash(password)
	if err != nil {
		return err
	}
	var result *gorm.DB
	err = s.Transaction(ctx, func(tx *gorm.DB) error {
		result = tx.Model(&model.User{}).Scopes(s.scoped(ctx)).Where("`"+s.TableName+"`.id = ?", id).Updates(map[string]interface{}{
			"password": hash,
		})
		return result.Error
	})
	if err != nil {
		return err
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdateStatus updates only the status field for a single user within the
// current actor's scope. It is used for switch-style partial edits so that
// the update carries scope and cannot touch out-of-scope rows.
func (s *UserRepository) UpdateStatus(ctx *gin.Context, id int32, status string) error {
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		result = tx.Model(&model.User{}).Scopes(s.scoped(ctx)).Where("`"+s.TableName+"`.id = ?", id).Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.User{}).Scopes(s.scoped(ctx)).Where("`"+s.TableName+"`.id = ?", id).Count(&visible).Error; err != nil {
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

func (s *UserRepository) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return fmt.Errorf("invalid user ids")
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return fmt.Errorf("invalid user id %d", id)
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var list []model.User
		scoped := tx.Model(&model.User{}).Scopes(s.scoped(ctx))
		// model.User deletion and balance changes use the same user-row lock protocol.
		if err := scoped.Clauses(clause.Locking{Strength: "UPDATE"}).Where("`"+s.TableName+"`.id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		// Reject deletion if any user still has money logs to prevent new orphans.
		moneyTable := s.config.Database.Prefix + "user_money_log"
		var moneyLogs []struct{ ID int32 }
		if err := tx.Table(moneyTable).Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("user_id IN ?", normalized).Find(&moneyLogs).Error; err != nil {
			return err
		}
		if len(moneyLogs) > 0 {
			return cErr.BadRequest("user has money logs, cannot delete")
		}
		del := scoped.Where("`"+s.TableName+"`.id IN ?", normalized).Delete(nil)
		if del.Error != nil {
			return del.Error
		}
		if del.RowsAffected != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
