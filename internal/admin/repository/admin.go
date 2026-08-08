package repository

import (
	"context"
	"errors"
	"strings"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	passwordutil "buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/persistence"
	"buildadmin-go/internal/pkg/util"

	"github.com/gin-gonic/gin"
	mysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type AdminRepository struct {
	persistence.BaseModel
	config *conf.Configuration
}

func NewAdminRepository(sqlDB *gorm.DB, config *conf.Configuration) *AdminRepository {
	return &AdminRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"admin", "id", "username,nickname", sqlDB),
		config:    config,
	}
}

// applyAdminDerived 应用与数据库无关的派生字段（头像默认值、邀请码）。
// 单行路径（DealData）与列表批处理路径（loadGroupSummaries 之后）共用，
// 保证两条路径的字段填充逐字段一致。邀请码为确定性派生
// （HMAC(token.key, admin id)），不落库、固定不可改。
func (s *AdminRepository) applyAdminDerived(data *model.Admin) {
	data.Avatar = util.DefaultUrl(data.Avatar, s.config.App.DefaultAvatar)
	data.InviteCode = util.InviteCode(s.config.Token.Key, data.ID)
}

func (s *AdminRepository) DealData(ctx context.Context, data *model.Admin) error {
	s.applyAdminDerived(data)

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

// loadGroupSummaries 批量装载列表管理员的组信息（GroupArr/GroupNameArr），
// 取代 listScoped 原先对每行执行一次 admin_group_access 查询的 N+1 形态。
// 与 loadParentSummaries 同构：收集全部行的 uid → 一次 WHERE uid IN 查询 →
// 按 uid 回填。逐行填充顺序沿用查询返回顺序（与逐行查询同一访问路径的
// 返回顺序一致），缺失组的行保持空数组，与逐行 DealData 输出一致。
func (s *AdminRepository) loadGroupSummaries(ctx context.Context, db *gorm.DB, admins []*model.Admin) error {
	ids := make([]int32, 0, len(admins))
	seen := make(map[int32]struct{}, len(admins))
	for _, admin := range admins {
		if admin == nil || admin.ID <= 0 {
			continue
		}
		if _, ok := seen[admin.ID]; !ok {
			seen[admin.ID] = struct{}{}
			ids = append(ids, admin.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	groups := []struct {
		Uid  int32
		Id   int32
		Name string
	}{}
	prefix := s.config.Database.Prefix
	if err := db.Session(&gorm.Session{NewDB: true}).Table(prefix+"admin_group_access").
		Joins("left join "+prefix+"admin_group g on g.id="+prefix+"admin_group_access.group_id").
		Select(prefix+"admin_group_access.uid as uid,g.id as id,g.name as name").
		Where(prefix+"admin_group_access.uid IN ?", ids).Scan(&groups).Error; err != nil {
		return err
	}
	byAdmin := make(map[int32]*model.Admin, len(admins))
	for _, admin := range admins {
		if admin != nil {
			byAdmin[admin.ID] = admin
		}
	}
	for _, v := range groups {
		if admin := byAdmin[v.Uid]; admin != nil {
			admin.GroupArr = append(admin.GroupArr, v.Id)
			admin.GroupNameArr = append(admin.GroupNameArr, v.Name)
		}
	}
	return nil
}

// actor extracts the authenticated data-scope actor. A nil context is treated
// as an unrestricted test/admin fixture; a non-nil context without a valid
// actor fails closed.
func (s *AdminRepository) actor(ctx *gin.Context) (data_scope.Actor, error) {
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
func (s *AdminRepository) scoped(ctx *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		enforcer := data_scope.NewClosureEnforcer(s.config)
		return enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: s.TableName, Column: "id"})
	}
}

// scopedWithActor is the transport-free counterpart of scoped for the
// service layer, which receives the actor as an explicit parameter.
func (s *AdminRepository) scopedWithActor(ctx context.Context, actor data_scope.Actor) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		enforcer := data_scope.NewClosureEnforcer(s.config)
		return enforcer.ScopeWithActor(ctx, db, actor, data_scope.OwnerRef{TableAlias: s.TableName, Column: "id"})
	}
}

func (s *AdminRepository) GetOne(ctx *gin.Context, id int32) (model.Admin, error) {
	actor, err := s.actor(ctx)
	if err != nil {
		return model.Admin{}, err
	}
	return s.GetOneWithActor(ctx, actor, id)
}

// GetOneWithActor loads one scoped administrator row for an explicit actor.
func (s *AdminRepository) GetOneWithActor(ctx context.Context, actor data_scope.Actor, id int32) (model.Admin, error) {
	data := model.Admin{}
	if err := s.DBFor(ctx).Scopes(s.scopedWithActor(ctx, actor)).Omit("password", "login_failure").Where("id=?", id).Limit(1).First(&data).Error; err != nil {
		return data, err
	}
	if err := s.DealData(ctx, &data); err != nil {
		return data, err
	}
	if err := s.loadParentSummaries(ctx, s.DBFor(ctx), s.scopedWithActor(ctx, actor), []*model.Admin{&data}); err != nil {
		return data, err
	}
	return data, nil
}

func (s *AdminRepository) GetGroupArr(ctx context.Context, id int32) (groupIds []int32, err error) {
	err = s.DBFor(ctx).Model(&model.AdminGroupAccess{}).Where("uid=?", id).Pluck("group_id", &groupIds).Error
	return
}

func (s *AdminRepository) GetGroupNameArr(ctx *gin.Context, id int32) (groupNames []string, err error) {
	prefix := s.config.Database.Prefix
	err = s.DBFor(ctx).Model(&model.AdminGroupAccess{}).
		Joins("left join "+prefix+"admin_group on "+prefix+"admin_group_access.group_id = "+prefix+"admin_group.id").Where("uid=?", id).Pluck("name", &groupNames).Error
	return
}

func (s *AdminRepository) List(ctx *gin.Context) (list []*model.Admin, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return
	}
	// count 与 find 必须使用独立 statement：复用同一 db 先 Count 再 Find 时，
	// GORM 的 Count 会重置 statement，导致 Find 丢失 scope/搜索 WHERE
	// （受限管理员看到上级数据的根因；与生成器产物 countDB/findDB 模式对齐）。
	countDB := s.DBFor(ctx).Model(&model.Admin{}).Scopes(s.scoped(ctx)).Where(whereS, whereP...)
	if err = countDB.Count(&total).Error; err != nil {
		return
	}
	list, err = s.listScoped(ctx, whereS, whereP, orderS, limit, offset)
	return
}

// ListTree 全量返回 scoped 管理员（不分页），供树状列表组装 children 使用。
// 与 List 共享 listScoped（count 与 find 各自独立 statement 的纪律同样适用）。
// 注意 GORM 语义：Limit(0) 会生成 `LIMIT 0`（0 行），Limit(-1) 才表示取消
// LIMIT 子句——全量查询必须传 -1。
func (s *AdminRepository) ListTree(ctx *gin.Context) (list []*model.Admin, err error) {
	whereS, whereP, orderS, _, _, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, err
	}
	return s.listScoped(ctx, whereS, whereP, orderS, -1, 0)
}

func (s *AdminRepository) listScoped(ctx *gin.Context, whereS string, whereP []interface{}, orderS string, limit, offset int) (list []*model.Admin, err error) {
	findDB := s.DBFor(ctx).Model(&model.Admin{}).Scopes(s.scoped(ctx)).Where(whereS, whereP...)
	err = findDB.Omit("password", "login_failure").Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	if err != nil {
		return nil, err
	}
	if err = s.loadParentSummaries(ctx, s.DBFor(ctx), s.scoped(ctx), list); err != nil {
		return nil, err
	}
	// 组信息批量装载（一次 WHERE uid IN 查询），替代逐行 DealData 的 N+1；
	// 派生字段（头像/邀请码）与数据库无关，逐行就地应用。
	if err = s.loadGroupSummaries(ctx, s.DBFor(ctx), list); err != nil {
		return nil, err
	}
	for _, v := range list {
		s.applyAdminDerived(v)
	}
	return list, nil
}

func (s *AdminRepository) loadParentSummaries(ctx context.Context, db *gorm.DB, scope func(*gorm.DB) *gorm.DB, admins []*model.Admin) error {
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
	var parents []model.AdminSummary
	if err := db.Session(&gorm.Session{NewDB: true}).Model(&model.Admin{}).Scopes(scope).Select("id", "nickname", "username").Where("id IN ?", ids).Find(&parents).Error; err != nil {
		return err
	}
	byID := make(map[int32]*model.AdminSummary, len(parents))
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

// AddWithActor writes a new administrator with its group assignments and the
// closure link. It is the scoped write protocol: uniqueness, group writes and
// the hierarchy link run in one caller-owned transaction.
func (s *AdminRepository) AddWithActor(ctx context.Context, admin model.Admin, groups []string, actor data_scope.Actor) error {
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	enforcer := data_scope.NewClosureEnforcer(s.config)
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("username=?", admin.Username).Take(&model.Admin{}).Error; err == nil {
			return cErr.BadRequest("username already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if result := tx.Omit("login_failure", "last_login_time", "last_login_ip").Create(&admin); result.Error != nil {
			if isDuplicateKeyError(result.Error) {
				return cErr.BadRequest("username already exists")
			}
			return result.Error
		} else if result.RowsAffected != 1 {
			return cErr.BadRequest("create failed: rows affected mismatch")
		}
		access := make([]map[string]interface{}, 0, len(groups))
		for _, v := range groups {
			access = append(access, map[string]interface{}{"uid": admin.ID, "group_id": v})
		}
		if len(access) > 0 {
			if result := tx.Model(&model.AdminGroupAccess{}).Create(access); result.Error != nil {
				return result.Error
			} else if result.RowsAffected != int64(len(access)) {
				return cErr.BadRequest("group assignment failed: rows affected mismatch")
			}
		}
		return NewAdminHierarchy(s.config).LinkNewNodeWithScope(ctx, tx, admin.ID, admin.ParentID, actor, enforcer)
	})
}

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

// CheckParentInScopeWithActor verifies that the requested parent administrator
// exists inside the actor's hierarchical scope. It fails closed: missing or
// unauthorized parents are treated as not found.
func (s *AdminRepository) CheckParentInScopeWithActor(ctx context.Context, actor data_scope.Actor, parentID int32) error {
	var parent model.Admin
	if err := s.DBFor(ctx).Scopes(s.scopedWithActor(ctx, actor)).Where("id = ?", parentID).First(&parent).Error; err != nil {
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
func (s *AdminRepository) SelectTree(ctx *gin.Context, excludeID int32, keyword string) ([]*model.Admin, error) {
	db := s.DBFor(ctx).Model(&model.Admin{}).Scopes(s.scoped(ctx)).Select("id", "parent_id", "nickname", "username")
	if keyword != "" {
		like := "%" + strings.Replace(keyword, "%", "\\%", -1) + "%"
		db = db.Where("nickname LIKE ? OR username LIKE ?", like, like)
	}
	if excludeID > 0 {
		closureTable := s.TableName + "_closure"
		sub := s.DBFor(ctx).Table(closureTable).Select("descendant_id").Where("ancestor_id = ?", excludeID)
		db = db.Where(s.TableName+".id NOT IN (?)", sub)
	}
	var list []*model.Admin
	if err := db.Order(s.TableName + ".id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// EditWithActor updates an administrator with parent/group changes. It is
// the scoped write protocol: uniqueness, hierarchy validation/move and group
// re-assignment run in one caller-owned transaction.
func (s *AdminRepository) EditWithActor(ctx context.Context, admin model.Admin, changeParent bool, newParent *int32, omit []string, groups []string, actor data_scope.Actor) error {
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	enforcer := data_scope.NewClosureEnforcer(s.config)
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("id<>? and username=?", admin.ID, admin.Username).Take(&model.Admin{}).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			return cErr.BadRequest("Account exist")
		}
		if err := NewAdminHierarchy(s.config).ValidateOrMoveWithScope(ctx, tx, admin.ID, changeParent, newParent, actor, enforcer); err != nil {
			return err
		}
		admin.ParentID = newParent
		if result := tx.Omit(append(omit, "parent_id")...).Save(&admin); result.Error != nil {
			return result.Error
		} else if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.Admin{}).Where("id = ?", admin.ID).Count(&visible).Error; err != nil {
				return err
			}
			if visible != 1 {
				return cErr.BadRequest("update failed: rows affected mismatch")
			}
		} else if result.RowsAffected != 1 {
			return cErr.BadRequest("update failed: rows affected mismatch")
		}
		if len(groups) > 0 {
			if err := tx.Model(&model.AdminGroupAccess{}).Where("uid=?", admin.ID).Delete(nil).Error; err != nil {
				return err
			}
			access := make([]map[string]interface{}, 0, len(groups))
			for _, v := range groups {
				access = append(access, map[string]interface{}{"uid": admin.ID, "group_id": v})
			}
			if result := tx.Model(&model.AdminGroupAccess{}).Create(access); result.Error != nil {
				return result.Error
			} else if result.RowsAffected != int64(len(access)) {
				return cErr.BadRequest("group assignment failed: rows affected mismatch")
			}
		}
		return nil
	})
}

// SwitchStatusWithActor performs a scoped, atomic status switch for a single
// administrator. The final UPDATE carries the closure scope predicate and
// validates RowsAffected.
func (s *AdminRepository) SwitchStatusWithActor(ctx context.Context, id int32, status string, actor data_scope.Actor) error {
	if status != "enable" && status != "disable" {
		return cErr.BadRequest("status must be enable or disable")
	}
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		result := tx.Model(&model.Admin{}).Scopes(s.scopedWithActor(ctx, actor)).Where("id = ?", id).Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.Admin{}).Scopes(s.scopedWithActor(ctx, actor)).Where("id = ?", id).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return cErr.BadRequest("record not found or no permission")
		}
		if result.RowsAffected != 1 {
			return cErr.BadRequest("record not found or no permission")
		}
		return nil
	})
}

func (s *AdminRepository) SelfEdit(ctx *gin.Context, admin model.Admin, selectField []string) error {
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		result := tx.Select(selectField).Save(&admin)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			var visible int64
			if err := tx.Model(&model.Admin{}).Where("id = ?", admin.ID).Count(&visible).Error; err != nil {
				return err
			}
			if visible == 1 {
				return nil
			}
			return cErr.BadRequest("update failed: rows affected mismatch")
		}
		if result.RowsAffected != 1 {
			return cErr.BadRequest("update failed: rows affected mismatch")
		}
		return nil
	})
}

// ResetPassword hashes and writes a new password for one administrator
// (single-statement scoped write with an existence fallback).
func (s *AdminRepository) ResetPassword(ctx *gin.Context, id int32, password string) error {
	hash, err := passwordutil.Hash(password)
	if err != nil {
		return err
	}
	result := s.DBFor(ctx).Model(&model.Admin{}).Where("id=?", id).Updates(map[string]interface{}{
		"password": hash,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var visible int64
		if err := s.DBFor(ctx).Model(&model.Admin{}).Where("id=?", id).Count(&visible).Error; err != nil {
			return err
		}
		if visible == 1 {
			return nil
		}
		return cErr.BadRequest("record not found")
	}
	if result.RowsAffected != 1 {
		return cErr.BadRequest("record not found")
	}
	return nil
}
