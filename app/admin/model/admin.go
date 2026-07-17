package model

import (
	"errors"
	"strings"

	"go-build-admin/app/pkg/data_scope"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"go-build-admin/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Admin struct {
	ID            int32         `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                                             // ID
	ParentID      *int32        `gorm:"column:parent_id;type:int(11) unsigned;default:null;comment:父级管理员ID;index:idx_parent_id" json:"parent_id"` // 父级管理员ID
	Username      string        `gorm:"column:username;not null;comment:用户名" json:"username"`                                                     // 用户名
	Nickname      string        `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`                                                      // 昵称
	Avatar        string        `gorm:"column:avatar;not null;comment:头像" json:"avatar"`                                                          // 头像
	Email         string        `gorm:"column:email;not null;comment:邮箱" json:"email"`                                                            // 邮箱
	Mobile        string        `gorm:"column:mobile;not null;comment:手机" json:"mobile"`                                                          // 手机
	LoginFailure  int32         `gorm:"column:login_failure;not null;comment:登录失败次数" json:"login_failure"`                                        // 登录失败次数
	LastLoginTime int64         `gorm:"column:last_login_time;comment:上次登录时间" json:"last_login_time"`                                             // 上次登录时间
	LastLoginIP   string        `gorm:"column:last_login_ip;not null;comment:上次登录IP" json:"last_login_ip"`                                        // 上次登录IP
	Password      string        `gorm:"column:password;not null;comment:密码" json:"password"`                                                      // 密码
	Salt          string        `gorm:"column:salt;not null;comment:密码盐" json:"salt"`                                                             // 密码盐
	Motto         string        `gorm:"column:motto;not null;comment:签名" json:"motto"`                                                            // 签名
	Status        string        `gorm:"column:status;type:varchar(30);not null;default:enable;comment:状态:enable=启用,disable=禁用" json:"status"`     // 状态:enable=启用,disable=禁用
	UpdateTime    int64         `gorm:"autoUpdateTime;column:update_time;comment:更新时间" json:"update_time"`                                        // 更新时间
	CreateTime    int64         `gorm:"autoCreateTime;column:create_time;comment:创建时间" json:"create_time"`                                        // 创建时间
	GroupArr      []int32       `gorm:"-" json:"group_arr"`
	GroupNameArr  []string      `gorm:"-" json:"group_name_arr"`
	Parent        *AdminSummary `gorm:"-" json:"parent,omitempty"`
}

type AdminSummary struct {
	ID       int32  `json:"id"`
	Nickname string `json:"nickname"`
}

type AdminModel struct {
	BaseModel
	config *conf.Configuration
}

func NewAdminModel(sqlDB *gorm.DB, config *conf.Configuration) *AdminModel {
	return &AdminModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "admin",
			Key:              "id",
			QuickSearchField: "username,nickname",
			sqlDB:            sqlDB,
		},
		config: config,
	}
}

func (s *AdminModel) DealData(ctx *gin.Context, data *Admin) error {
	data.Avatar = utils.DefaultUrl(data.Avatar, s.config.App.DefaultAvatar)

	groups := []struct {
		Id   int32
		Name string
	}{}
	prefix := s.config.Database.Prefix
	err := s.DBFor(ctx).Table(prefix+"admin_group_access").
		Joins("left join "+prefix+"admin_group g on g.id="+prefix+"admin_group_access.group_id").
		Select("g.id as id,g.name as name").
		Where(prefix+"admin_group_access.uid=?", data.ID).Scan(&groups).Error

	if err != nil {
		return err
	}
	for _, v := range groups {
		data.GroupArr = append(data.GroupArr, v.Id)
		data.GroupNameArr = append(data.GroupNameArr, v.Name)
	}
	return nil
}

// actor extracts the authenticated data-scope actor. A nil context is treated
// as an unrestricted test/admin fixture; a non-nil context without a valid
// actor fails closed.
func (s *AdminModel) actor(ctx *gin.Context) (data_scope.Actor, error) {
	if ctx == nil || ctx.Request == nil {
		return data_scope.Actor{}, data_scope.ErrScopedAccessDenied
	}
	actor, ok := data_scope.ActorFromContext(ctx)
	if !ok || data_scope.ValidateActor(actor) != nil {
		return data_scope.Actor{}, data_scope.ErrScopedAccessDenied
	}
	return actor, nil
}

// scoped applies the fail-closed hierarchical data-scope enforcer to admin.id.
// Only an explicit unrestricted actor bypasses scope; every other actor sees
// only self and descendants. The returned DB carries ErrScopedAccessDenied
// when the actor is missing, invalid, or the closure table is unavailable.
func (s *AdminModel) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		enforcer := data_scope.NewClosureEnforcer(s.config)
		return enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "id"})
	}
}

func (s *AdminModel) GetOne(ctx *gin.Context, id int32) (Admin, error) {
	data := Admin{}
	if err := s.DBFor(ctx).Scopes(s.scoped(ctx)).Omit("password", "salt", "login_failure").Where("id=?", id).Limit(1).First(&data).Error; err != nil {
		return data, err
	}
	if err := s.DealData(ctx, &data); err != nil {
		return data, err
	}
	if err := s.loadParentSummaries(ctx, s.DBFor(ctx), []*Admin{&data}); err != nil {
		return data, err
	}
	return data, nil
}

func (s *AdminModel) GetGroupArr(ctx *gin.Context, id int32) (groupIds []int32, err error) {
	err = s.DBFor(ctx).Model(&AdminGroupAccess{}).Where("uid=?", id).Pluck("group_id", &groupIds).Error
	return
}

func (s *AdminModel) GetGroupNameArr(ctx *gin.Context, id int32) (groupNames []string, err error) {
	prefix := s.config.Database.Prefix
	err = s.DBFor(ctx).Model(&AdminGroupAccess{}).
		Joins("left join "+prefix+"admin_group on "+prefix+"admin_group_access.group_id = "+prefix+"admin_group.id").Where("uid=?", id).Pluck("name", &groupNames).Error
	return
}

func (s *AdminModel) List(ctx *gin.Context) (list []*Admin, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return
	}
	db := s.DBFor(ctx).Model(&Admin{}).Scopes(s.scoped(ctx)).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return
	}
	err = db.Omit("password", "salt", "login_failure").Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	if err != nil {
		return
	}
	if err = s.loadParentSummaries(ctx, db, list); err != nil {
		return
	}
	for _, v := range list {
		if err = s.DealData(ctx, v); err != nil {
			return
		}
	}
	return
}

func (s *AdminModel) loadParentSummaries(ctx *gin.Context, db *gorm.DB, admins []*Admin) error {
	ids := make([]int32, 0, len(admins))
	seen := make(map[int32]struct{}, len(admins))
	for _, admin := range admins {
		if admin == nil || admin.ParentID == nil || *admin.ParentID <= 0 {
			continue
		}
		if _, ok := seen[*admin.ParentID]; !ok {
			seen[*admin.ParentID] = struct{}{}
			ids = append(ids, *admin.ParentID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var parents []AdminSummary
	if err := db.Session(&gorm.Session{NewDB: true}).Model(&Admin{}).Scopes(s.scoped(ctx)).Select("id", "nickname").Where("id IN ?", ids).Find(&parents).Error; err != nil {
		return err
	}
	byID := make(map[int32]*AdminSummary, len(parents))
	for i := range parents {
		byID[parents[i].ID] = &parents[i]
	}
	for _, admin := range admins {
		if admin != nil && admin.ParentID != nil {
			admin.Parent = byID[*admin.ParentID]
		}
	}
	return nil
}

func (s *AdminModel) Add(ctx *gin.Context, admin Admin, groups []string) error {
	actor, err := s.actor(ctx)
	if err != nil {
		return err
	}
	enforcer := data_scope.NewClosureEnforcer(s.config)
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("username=?", admin.Username).Take(&Admin{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			return cErr.BadRequest("Account exist")
		}
		if result := tx.Omit("login_failure", "last_login_time", "last_login_ip").Create(&admin); result.Error != nil {
			return result.Error
		} else if result.RowsAffected != 1 {
			return cErr.BadRequest("create failed: rows affected mismatch")
		}
		access := make([]map[string]interface{}, 0, len(groups))
		for _, v := range groups {
			access = append(access, map[string]interface{}{"uid": admin.ID, "group_id": v})
		}
		if len(access) > 0 {
			if result := tx.Model(&AdminGroupAccess{}).Create(access); result.Error != nil {
				return result.Error
			} else if result.RowsAffected != int64(len(access)) {
				return cErr.BadRequest("group assignment failed: rows affected mismatch")
			}
		}
		return NewAdminHierarchy(s.config).LinkNewNodeWithScope(ctx, tx, admin.ID, admin.ParentID, actor, enforcer)
	})
}

// CheckParentInScope verifies that the requested parent administrator exists
// inside the current actor's hierarchical scope. It fails closed: missing or
// unauthorized parents are treated as not found.
func (s *AdminModel) CheckParentInScope(ctx *gin.Context, parentID int32) error {
	var parent Admin
	if err := s.DBFor(ctx).Scopes(s.scoped(ctx)).Where("id = ?", parentID).First(&parent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, data_scope.ErrScopedAccessDenied) {
			return cErr.BadRequest("Parent administrator not found or not in scope")
		}
		return err
	}
	return nil
}

// SelectTree returns the administrators visible to the current actor for use
// as a parent selector. Results are scoped to self and descendants. The
// exclude_id subtree is removed using the closure table so keyword filtering
// cannot leak excluded descendants.
func (s *AdminModel) SelectTree(ctx *gin.Context, excludeID int32, keyword string) ([]*Admin, error) {
	db := s.DBFor(ctx).Model(&Admin{}).Scopes(s.scoped(ctx)).Select("id", "parent_id", "nickname", "username")
	if keyword != "" {
		like := "%" + strings.Replace(keyword, "%", "\\%", -1) + "%"
		db = db.Where("nickname LIKE ? OR username LIKE ?", like, like)
	}
	if excludeID > 0 {
		closureTable := s.TableName + "_closure"
		sub := s.DBFor(ctx).Table(closureTable).Select("descendant_id").Where("ancestor_id = ?", excludeID)
		db = db.Where(s.TableName+".id NOT IN (?)", sub)
	}
	var list []*Admin
	if err := db.Order(s.TableName + ".id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (s *AdminModel) Edit(ctx *gin.Context, admin Admin, changeParent bool, newParent *int32, omit []string, groups []string) error {
	actor, err := s.actor(ctx)
	if err != nil {
		return err
	}
	enforcer := data_scope.NewClosureEnforcer(s.config)
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("id<>? and username=?", admin.ID, admin.Username).Take(&Admin{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			return cErr.BadRequest("Account exist")
		}
		if err := NewAdminHierarchy(s.config).ValidateOrMoveWithScope(ctx, tx, admin.ID, changeParent, newParent, actor, enforcer); err != nil {
			return err
		}
		admin.ParentID = newParent
		if result := tx.Omit(append(omit, "parent_id")...).Save(&admin); result.Error != nil {
			return result.Error
		} else if result.RowsAffected != 1 {
			return cErr.BadRequest("update failed: rows affected mismatch")
		}
		if len(groups) > 0 {
			if err := tx.Model(&AdminGroupAccess{}).Where("uid=?", admin.ID).Delete(nil).Error; err != nil {
				return err
			}
			access := make([]map[string]interface{}, 0, len(groups))
			for _, v := range groups {
				access = append(access, map[string]interface{}{"uid": admin.ID, "group_id": v})
			}
			if result := tx.Model(&AdminGroupAccess{}).Create(access); result.Error != nil {
				return result.Error
			} else if result.RowsAffected != int64(len(access)) {
				return cErr.BadRequest("group assignment failed: rows affected mismatch")
			}
		}
		return nil
	})
}

// SwitchStatus performs a scoped, atomic status switch for a single
// administrator. The final UPDATE carries the closure scope predicate and
// validates RowsAffected.
func (s *AdminModel) SwitchStatus(ctx *gin.Context, id int32, status string) error {
	if status != "enable" && status != "disable" {
		return cErr.BadRequest("status must be enable or disable")
	}
	if _, err := s.actor(ctx); err != nil {
		return err
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		result := tx.Model(&Admin{}).Scopes(s.scoped(ctx)).Where("id = ?", id).Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return cErr.BadRequest("record not found or no permission")
		}
		return nil
	})
}

func (s *AdminModel) SelfEdit(ctx *gin.Context, admin Admin, selectField []string) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		result := tx.Select(selectField).Save(&admin)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return cErr.BadRequest("update failed: rows affected mismatch")
		}
		return nil
	})
}

func (s *AdminModel) ResetPassword(ctx *gin.Context, id int32, password string) error {
	salt := random.Build("alnum", 16)
	password = utils.EncryptPassword(password, salt)
	result := s.DBFor(ctx).Model(&Admin{}).Where("id=?", id).Updates(map[string]interface{}{
		"salt":     salt,
		"password": password,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return cErr.BadRequest("record not found")
	}
	return nil
}

func normalizeAdminIDs(ids []int32) ([]int32, error) {
	seen := make(map[int32]struct{}, len(ids))
	out := make([]int32, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, cErr.BadRequest("ids must be positive")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func (s *AdminModel) Del(ctx *gin.Context, ids interface{}) error {
	idList, ok := ids.([]int32)
	if !ok {
		return cErr.BadRequest("invalid ids")
	}
	idList, err := normalizeAdminIDs(idList)
	if err != nil {
		return err
	}
	if len(idList) == 0 {
		return nil
	}
	actor, err := s.actor(ctx)
	if err != nil {
		return err
	}
	enforcer := data_scope.NewClosureEnforcer(s.config)
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		return NewAdminHierarchy(s.config).DeleteAdmins(ctx, tx, idList, actor, enforcer)
	})
}
