package model

import (
	"errors"
	"fmt"
	"go-build-admin/app/admin/model/simple"
	"go-build-admin/app/pkg/data_scope"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// User 会员表
type User struct {
	ID            int32        `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                         // ID
	AdminID       int32        `gorm:"column:admin_id;not null;comment:管理员ID" json:"admin_id"`                                               // 管理员ID
	GroupID       int32        `gorm:"column:group_id;not null;comment:分组ID" json:"group_id"`                                                // 分组ID
	Username      string       `gorm:"column:username;not null;comment:用户名" json:"username"`                                                 // 用户名
	Nickname      string       `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`                                                  // 昵称
	Email         string       `gorm:"column:email;not null;comment:邮箱" json:"email"`                                                        // 邮箱
	Mobile        string       `gorm:"column:mobile;not null;comment:手机" json:"mobile"`                                                      // 手机
	Avatar        string       `gorm:"column:avatar;not null;comment:头像" json:"avatar"`                                                      // 头像
	Gender        int32        `gorm:"column:gender;not null;comment:性别:0=未知,1=男,2=女" json:"gender"`                                         // 性别:0=未知,1=男,2=女
	Birthday      time.Time    `gorm:"column:birthday;comment:生日" json:"birthday"`                                                           // 生日
	Money         int32        `gorm:"column:money;not null;comment:余额" json:"money"`                                                        // 余额
	Score         int32        `gorm:"column:score;not null;comment:积分" json:"score"`                                                        // 积分
	LastLoginTime int64        `gorm:"column:last_login_time;comment:上次登录时间" json:"last_login_time"`                                         // 上次登录时间
	LastLoginIP   string       `gorm:"column:last_login_ip;not null;comment:上次登录IP" json:"last_login_ip"`                                    // 上次登录IP
	LoginFailure  int32        `gorm:"column:login_failure;not null;comment:登录失败次数" json:"login_failure"`                                    // 登录失败次数
	JoinIP        string       `gorm:"column:join_ip;not null;comment:加入IP" json:"join_ip"`                                                  // 加入IP
	JoinTime      int64        `gorm:"column:join_time;comment:加入时间" json:"join_time"`                                                       // 加入时间
	Motto         string       `gorm:"column:motto;not null;comment:签名" json:"motto"`                                                        // 签名
	Password      string       `gorm:"column:password;not null;comment:密码" json:"password"`                                                  // 密码
	Salt          string       `gorm:"column:salt;not null;comment:密码盐" json:"salt"`                                                         // 密码盐
	Status        string       `gorm:"column:status;type:varchar(30);not null;default:enable;comment:状态:enable=启用,disable=禁用" json:"status"` // 状态:enable=启用,disable=禁用
	UpdateTime    int64        `gorm:"autoCreateTime;column:update_time;comment:更新时间" json:"update_time"`                                    // 更新时间
	CreateTime    int64        `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`                                    // 创建时间
	Admin         simple.Admin `gorm:"foreignKey:AdminID" json:"admin"`
	Group         UserGroup    `gorm:"foreignKey:GroupID" json:"group"`
}

type OutUser struct {
	User
	Birthday string `json:"birthday"`
	Money    string `json:"money"`
}

type UserModel struct {
	BaseModel
	config   *conf.Configuration
	enforcer data_scope.Enforcer
}

func NewUserModel(sqlDB *gorm.DB, config *conf.Configuration, enforcer data_scope.Enforcer) *UserModel {
	return &UserModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "user",
			Key:              "id",
			QuickSearchField: "username,nickname",
			sqlDB:            sqlDB,
		},
		config:   config,
		enforcer: enforcer,
	}
}

func (s *UserModel) DealData(ctx *gin.Context, data *User) (*OutUser, error) {
	outUser := OutUser{}
	if err := copier.Copy(&outUser, data); err != nil {
		return nil, err
	}
	outUser.Avatar = utils.DefaultUrl(data.Avatar, s.config.App.DefaultAvatar)
	outUser.Money = fmt.Sprintf("%.2f", float64(data.Money)/100)
	outUser.Birthday = ""
	if data.Birthday.Unix() > 100 {
		outUser.Birthday = data.Birthday.Format("2006-01-02")
	}
	return &outUser, nil
}

// scoped applies the fail-closed hierarchical data-scope enforcer to
// user.admin_id. Only an explicit unrestricted actor bypasses scope.
func (s *UserModel) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if s.enforcer == nil {
			tx := db.Session(&gorm.Session{})
			_ = tx.AddError(data_scope.ErrScopedAccessDenied)
			return tx
		}
		return s.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "admin_id"})
	}
}

func (s *UserModel) GetOne(ctx *gin.Context, id int32) (User, error) {
	data := User{}
	err := s.DBFor(ctx).Model(&User{}).Scopes(s.scoped(ctx)).Preload("Admin").Omit("password", "salt").Where("`"+s.TableName+"`.id = ?", id).First(&data).Error
	return data, err
}

func (s *UserModel) List(ctx *gin.Context) ([]*OutUser, int64, error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	var total int64 = 0
	list := []*User{}

	db := s.DBFor(ctx).Model(&User{}).Joins("Group").Where(whereS, whereP...)
	db = db.Preload("Admin").Preload("Group")
	db = db.Scopes(s.scoped(ctx))
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Omit("password", "salt").Order(orderS).Limit(limit).Offset(offset).Find(&list).Error; err != nil {
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

func (s *UserModel) Add(ctx *gin.Context, user *User) error {
	if s.enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	actor, err := s.enforcer.Actor(ctx)
	if err != nil {
		return err
	}
	user.AdminID = actor.AdminID

	return s.Transaction(ctx, func(tx *gorm.DB) error {
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
		if err := tx.Where("username=?", user.Username).Take(&User{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			return cErr.BadRequest("Account exist")
		}
		return tx.Omit("login_failure", "last_login_time", "last_login_ip").Create(user).Error
	})
}

func (s *UserModel) Edit(ctx *gin.Context, user *User, password string) error {
	updates := map[string]interface{}{
		"group_id":  user.GroupID,
		"username":  user.Username,
		"nickname":  user.Nickname,
		"email":     user.Email,
		"mobile":    user.Mobile,
		"avatar":    user.Avatar,
		"gender":    user.Gender,
		"birthday":  user.Birthday,
		"join_ip":   user.JoinIP,
		"join_time": user.JoinTime,
		"motto":     user.Motto,
		"status":    user.Status,
	}
	if password != "" {
		salt := random.Build("alnum", 16)
		updates["salt"] = salt
		updates["password"] = utils.EncryptPassword(password, salt)
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("id<>? and username=?", user.ID, user.Username).Take(&User{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			return cErr.BadRequest("Account not exist")
		}
		result := tx.Model(&User{}).Scopes(s.scoped(ctx)).Where("`"+s.TableName+"`.id = ?", user.ID).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (s *UserModel) ResetPassword(ctx *gin.Context, id int32, password string) error {
	salt := random.Build("alnum", 16)
	password = utils.EncryptPassword(password, salt)
	var result *gorm.DB
	err := s.Transaction(ctx, func(tx *gorm.DB) error {
		result = tx.Model(&User{}).Scopes(s.scoped(ctx)).Where("`"+s.TableName+"`.id = ?", id).Updates(map[string]interface{}{
			"salt":     salt,
			"password": password,
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
func (s *UserModel) UpdateStatus(ctx *gin.Context, id int32, status string) error {
	var result *gorm.DB
	if err := s.Transaction(ctx, func(tx *gorm.DB) error {
		result = tx.Model(&User{}).Scopes(s.scoped(ctx)).Where("`"+s.TableName+"`.id = ?", id).Update("status", status)
		return result.Error
	}); err != nil {
		return err
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *UserModel) Del(ctx *gin.Context, ids interface{}) error {
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
		var list []User
		scoped := tx.Model(&User{}).Scopes(s.scoped(ctx))
		// User deletion and balance/score changes use the same user-row lock
		// protocol.  Lock every requested row before consulting the log tables.
		if err := scoped.Clauses(clause.Locking{Strength: "UPDATE"}).Where("`"+s.TableName+"`.id IN ?", normalized).Find(&list).Error; err != nil {
			return err
		}
		if len(list) != len(normalized) {
			return gorm.ErrRecordNotFound
		}
		// Reject deletion if any user still has money/score logs to prevent new orphans.
		moneyTable := s.config.Database.Prefix + "user_money_log"
		scoreTable := s.config.Database.Prefix + "user_score_log"
		var moneyLogs []struct{ ID int32 }
		if err := tx.Table(moneyTable).Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("user_id IN ?", normalized).Find(&moneyLogs).Error; err != nil {
			return err
		}
		var scoreLogs []struct{ ID int32 }
		if err := tx.Table(scoreTable).Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("user_id IN ?", normalized).Find(&scoreLogs).Error; err != nil {
			return err
		}
		if len(moneyLogs)+len(scoreLogs) > 0 {
			return cErr.BadRequest("user has money or score logs, cannot delete")
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
