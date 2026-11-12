package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	passwordutil "buildadmin-go/internal/pkg/password"

	"github.com/jinzhu/copier"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// AdminService 承载管理员管理的业务规则：口令处理、用户名唯一性、分组/规则
// 装配、父级解析与可见性校验、状态流转与删除编排。handler 只做参数绑定与
// 响应返回。
type AdminService struct {
	adminM *adminmodel.AdminRepository
	authM  *adminmodel.AuthRepository
	config *conf.Configuration
}

func NewAdminService(adminM *adminmodel.AdminRepository, authM *adminmodel.AuthRepository, config *conf.Configuration) *AdminService {
	return &AdminService{adminM: adminM, authM: authM, config: config}
}

// ParentSelection is the transport-free form of the handler's presence-aware
// parent_id JSON value: Set reports whether the field was present at all.
type ParentSelection struct {
	Value *int32
	Set   bool
}

// AdminParams carries the plain (non-gin-bound) shape of an admin create or
// update request.
type AdminParams struct {
	Username string
	Nickname string
	Avatar   string
	Email    string
	Mobile   string
	Password string
	Motto    string
	Status   string
	ParentID ParentSelection
	GroupArr []string
}

// ResolveParentIDForAdd implements the Add semantics: omitted/null/0 means
// default to the current actor for restricted actors, or root for Unrestricted.
func (s *AdminService) ResolveParentIDForAdd(p ParentSelection, actor data_scope.Actor) (*int32, error) {
	if !p.Set || p.Value == nil || *p.Value == 0 {
		if actor.Unrestricted {
			return nil, nil
		}
		return &actor.AdminID, nil
	}
	if *p.Value < 0 {
		return nil, cErr.BadRequest("parent_id must be non-negative")
	}
	return p.Value, nil
}

// ResolveParentIDForEdit implements the Edit semantics: omitted/null keeps the
// current parent; 0 moves to root only for Unrestricted actors; positive values
// move to that parent. The returned bool indicates whether the parent changed.
func (s *AdminService) ResolveParentIDForEdit(p ParentSelection, current *int32, actor data_scope.Actor) (*int32, bool, error) {
	if !p.Set || p.Value == nil {
		return current, false, nil
	}
	if *p.Value == 0 {
		if !actor.Unrestricted {
			return nil, false, cErr.BadRequest("restricted actor cannot move administrator to root")
		}
		return nil, !int32PtrEqual(current, nil), nil
	}
	if *p.Value < 0 {
		return nil, false, cErr.BadRequest("parent_id must be non-negative")
	}
	return p.Value, !int32PtrEqual(current, p.Value), nil
}

func int32PtrEqual(a, b *int32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// IsMovingUnderSelf reports whether an existing node would be moved under
// itself. For a new administrator (nodeID == 0) this is always false because
// the node does not exist yet and the actor may legitimately create a direct
// subordinate.
func (s *AdminService) IsMovingUnderSelf(nodeID int32, parentID *int32) bool {
	return nodeID > 0 && parentID != nil && *parentID == nodeID
}

// CheckGroupAuth verifies that the operator may assign every requested group.
func (s *AdminService) CheckGroupAuth(groups []string, operatorID int32) error {
	if s.authM.IsSuperAdmin(operatorID) {
		return nil
	}

	authGroups, err := s.authM.GetAllAuthGroups("allAuthAndOthers", operatorID)
	if err != nil {
		return err
	}
	for _, v := range groups {
		if !slicesContains(authGroups, v) {
			return cErr.BadRequest("You have no permission to add an administrator to this group!")
		}
	}
	return nil
}

func slicesContains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

// CheckParentInScope validates that the requested parent administrator exists
// and is visible to the actor. Missing and out-of-scope parents collapse into
// the same fail-closed rejection, which is why the message carries both cases.
func (s *AdminService) CheckParentInScope(ctx context.Context, actor data_scope.Actor, parentID int32) error {
	visible, err := s.adminM.ParentVisibleWithActor(ctx, actor, parentID)
	if err != nil {
		return err
	}
	if !visible {
		return cErr.BadRequest("Parent administrator not found or not in scope")
	}
	return nil
}

// Add runs the whole administrator-creation flow: group authorization,
// parent resolution, password hashing, username uniqueness and the
// transactional write.
func (s *AdminService) Add(ctx context.Context, p AdminParams, actor data_scope.Actor, operatorID int32) error {
	if p.Password == "" {
		return cErr.BadRequest("Please input correct password")
	}
	if len(p.GroupArr) > 0 {
		if err := s.CheckGroupAuth(p.GroupArr, operatorID); err != nil {
			return err
		}
	}

	parentID, err := s.ResolveParentIDForAdd(p.ParentID, actor)
	if err != nil {
		return err
	}
	if parentID != nil {
		if err := s.CheckParentInScope(ctx, actor, *parentID); err != nil {
			return err
		}
	}

	var admin model.Admin
	if err := copier.Copy(&admin, p); err != nil {
		return err
	}
	hash, err := passwordutil.Hash(p.Password)
	if err != nil {
		return err
	}
	admin.Password = hash
	admin.ParentID = parentID

	// 唯一性预检在写入前完成（并发插入仍由 repo 的 duplicate-key 兜底），
	// 保证先出现口令错误，与原先“散列在事务前、预检在事务内”的文案顺序一致。
	exists, err := s.adminM.UsernameExists(ctx, admin.Username, 0)
	if err != nil {
		return err
	}
	if exists {
		return cErr.BadRequest("username already exists")
	}

	return s.adminM.AddWithActor(ctx, admin, p.GroupArr, actor)
}

// Edit runs the whole administrator-update flow: reload, self-disable guard,
// parent resolution, group delta authorization, password hashing, username
// uniqueness and the transactional write.
func (s *AdminService) Edit(ctx context.Context, id int32, p AdminParams, actor data_scope.Actor, operatorID int32) error {
	admin, err := s.adminM.GetOneWithActor(ctx, actor, id)
	if err != nil {
		return err
	}

	if operatorID == admin.ID && p.Status == "disable" {
		return cErr.BadRequest("Please use another administrator account to disable the current account!")
	}

	parentID, changed, err := s.ResolveParentIDForEdit(p.ParentID, admin.ParentID, actor)
	if err != nil {
		return err
	}
	if s.IsMovingUnderSelf(admin.ID, parentID) {
		return cErr.BadRequest("cannot move an administrator under itself")
	}

	omit := []string{"login_failure", "last_login_time", "parent_id"}
	if p.Password == "" {
		omit = append(omit, "password")
	}

	checkGroups := []string{}
	groupIds, _ := s.adminM.GetGroupArr(ctx, operatorID)
	for _, v := range p.GroupArr {
		for _, i := range groupIds {
			if v != strconv.Itoa(int(i)) {
				checkGroups = append(checkGroups, v)
			}
		}
	}
	if len(checkGroups) > 0 {
		if err := s.CheckGroupAuth(checkGroups, operatorID); err != nil {
			return err
		}
	}

	if changed && parentID != nil {
		if err := s.CheckParentInScope(ctx, actor, *parentID); err != nil {
			return err
		}
	}

	if err := copier.Copy(&admin, p); err != nil {
		return err
	}
	// Hash only after copier.Copy: the params password is plaintext and must
	// never survive into the model passed to the transactional writer.
	if p.Password != "" {
		hash, err := passwordutil.Hash(p.Password)
		if err != nil {
			return err
		}
		admin.Password = hash
	}
	admin.ParentID = parentID

	// 唯一性预检保留原文案 "Account exist"；并发插入由 repo 的 duplicate-key
	// 兜底，检查放在散列之后以保持与原实现相同的错误优先级。
	exists, err := s.adminM.UsernameExists(ctx, admin.Username, admin.ID)
	if err != nil {
		return err
	}
	if exists {
		return cErr.BadRequest("Account exist")
	}

	return s.adminM.EditWithActor(ctx, admin, changed, parentID, omit, p.GroupArr, actor)
}

// SelfAdminParams 是本人资料更新的纯参数形态：头像与昵称保留"是否提交"
// 语义，nil 表示请求未提供该字段。
type SelfAdminParams struct {
	Nickname *string
	Avatar   *string
	Email    string
	Mobile   string
	Password string
	Motto    string
}

// selfEditPlan 返回本人资料更新的字段白名单：提交了头像时只写 avatar（头像
// 更新早于且独立于其余资料，保持历史语义）；否则写资料字段，并在提供口令时
// 追加 password 字段（散列由 UpdateSelf 完成），保证口令与资料在同一条
// UPDATE 内落库。
func selfEditPlan(p SelfAdminParams) (avatarOnly bool, fields []string) {
	if p.Avatar != nil {
		return true, []string{"avatar"}
	}
	fields = []string{"nickname", "email", "mobile", "motto"}
	if p.Password != "" {
		fields = append(fields, "password")
	}
	return false, fields
}

// UpdateSelf 编排本人资料更新：先按 actor 加载本人记录，再按 selfEditPlan
// 组装字段并以单条 UPDATE 落库（提供口令时在同一条语句内写入散列值）。
func (s *AdminService) UpdateSelf(ctx context.Context, id int32, p SelfAdminParams, actor data_scope.Actor) error {
	admin, err := s.adminM.GetOneWithActor(ctx, actor, id)
	if err != nil {
		return err
	}

	avatarOnly, fields := selfEditPlan(p)
	if avatarOnly {
		admin.Avatar = *p.Avatar
		return s.adminM.SelfEdit(ctx, admin, fields)
	}

	if p.Nickname != nil {
		admin.Nickname = *p.Nickname
	}
	admin.Email = p.Email
	admin.Mobile = p.Mobile
	admin.Motto = p.Motto
	if p.Password != "" {
		hash, err := passwordutil.Hash(p.Password)
		if err != nil {
			return err
		}
		admin.Password = hash
	}
	return s.adminM.SelfEdit(ctx, admin, fields)
}

// SwitchStatus validates and performs a scoped status switch, refusing to
// disable the operator's own account.
func (s *AdminService) SwitchStatus(ctx context.Context, id int32, status string, operatorID int32, actor data_scope.Actor) error {
	if err := ValidateAccountStatusValue(status); err != nil {
		return err
	}
	if operatorID == id && status == "disable" {
		return cErr.BadRequest("Please use another administrator account to disable the current account!")
	}
	return s.adminM.SwitchStatusWithActor(ctx, id, status, actor)
}

// Del runs the whole scoped administrator-deletion flow: id normalization,
// actor validation, then one transaction that locks the hierarchy, re-verifies
// scope, rejects targets with subordinates, resolves and validates each
// target's reassignment parent, transfers owner rows and deletes the admin and
// closure rows. The audit log is emitted after commit.
func (s *AdminService) Del(ctx context.Context, ids []int32, actor data_scope.Actor) error {
	idList, err := normalizeIDs(ids, "admin", false, func(id int32) error {
		return cErr.BadRequest("ids must be positive")
	})
	if err != nil {
		return err
	}
	if len(idList) == 0 {
		return nil
	}
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	hierarchy := adminmodel.NewAdminHierarchy(s.config)
	enforcer := data_scope.NewClosureEnforcer(s.config)
	var result *adminmodel.AdminDeleteResult
	if err := s.adminM.Transaction(ctx, func(tx *gorm.DB) error {
		var txErr error
		result, txErr = s.deleteAdminsTx(ctx, tx, hierarchy, idList, actor, enforcer)
		return txErr
	}); err != nil {
		return err
	}
	if result != nil {
		zap.L().Info("administrator deleted",
			zap.Int32s("deleted_admin_ids", result.Deleted),
			zap.Any("owner_transfers", result.Transfers),
		)
	}
	return nil
}

// deleteAdminsTx 是 Del 的事务编排体。关系仍存在于闭包时，父级不可能与被删
// 节点同批（否则父级有下级，早被子级检查拒绝）；这里额外拒绝"父级也在删除
// 集合"的异常闭包，避免转移后立刻再次成为孤儿。parent_id 是框架后加的列，
// 从 PHP 上游升级的库不会回填，因此仅当管理员在已登记的归属表中确实拥有
// 数据时才要求有效的转移目标；顶层且无归属数据的管理员可直接删除。所有
// 目标的校验先于任何写入完成，避免部分转移后才发现另一目标不可删。
func (s *AdminService) deleteAdminsTx(ctx context.Context, tx *gorm.DB, hierarchy *adminmodel.AdminHierarchy, ids []int32, actor data_scope.Actor, enforcer *data_scope.ClosureEnforcer) (*adminmodel.AdminDeleteResult, error) {
	if err := hierarchy.LockHierarchyTx(tx); err != nil {
		return nil, err
	}
	if err := hierarchy.VerifyInScopeTx(ctx, tx, actor, enforcer, ids...); err != nil {
		return nil, err
	}

	childCount, err := hierarchy.CountSubordinatesTx(ctx, tx, ids...)
	if err != nil {
		return nil, err
	}
	if childCount > 0 {
		return nil, cErr.BadRequest("cannot delete administrator with subordinates")
	}

	targets, err := hierarchy.DeleteTargetsTx(ctx, tx, ids)
	if err != nil {
		return nil, err
	}
	deleting := make(map[int32]struct{}, len(targets))
	for _, target := range targets {
		deleting[target.ID] = struct{}{}
	}

	reassign := make(map[int32]int32, len(targets))
	for _, target := range targets {
		owns, err := hierarchy.AdminOwnsRowsTx(ctx, tx, target.ID)
		if err != nil {
			return nil, err
		}
		if !owns {
			continue
		}
		if target.ParentID == nil || *target.ParentID <= 0 {
			return nil, cErr.BadRequest(fmt.Sprintf("cannot delete top-level administrator %d without a reassignment target", target.ID))
		}
		parentID := *target.ParentID
		if _, alsoDeleting := deleting[parentID]; alsoDeleting {
			return nil, cErr.BadRequest(fmt.Sprintf("cannot delete administrator %d: its parent %d is also being deleted", target.ID, parentID))
		}
		if err := hierarchy.EnsureNodeExistsTx(ctx, tx, parentID); err != nil {
			return nil, err
		}
		if err := hierarchy.EnsureReassignmentTargetEnabledTx(ctx, tx, parentID); err != nil {
			if errors.Is(err, adminmodel.ErrReassignmentTargetDisabled) {
				return nil, cErr.BadRequest(fmt.Sprintf("reassignment target administrator %d is not enabled", parentID))
			}
			return nil, err
		}
		reassign[target.ID] = parentID
	}

	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	result := &adminmodel.AdminDeleteResult{Deleted: sorted}
	ownerTables := hierarchy.OwnerTables()
	for _, target := range targets {
		newOwnerID, ok := reassign[target.ID]
		if !ok {
			continue
		}
		for _, table := range ownerTables {
			rows, err := hierarchy.TransferOwnerRowsTx(ctx, tx, table, target.ID, newOwnerID)
			if err != nil {
				return nil, err
			}
			if rows == 0 {
				continue
			}
			result.Transfers = append(result.Transfers, adminmodel.AdminOwnerTransfer{
				AdminID:    target.ID,
				NewOwnerID: newOwnerID,
				Table:      table,
				Rows:       rows,
			})
		}
	}

	if err := hierarchy.DeleteAdminsTx(ctx, tx, sorted); err != nil {
		return nil, err
	}
	if err := hierarchy.DeleteClosureTx(ctx, tx, sorted); err != nil {
		return nil, err
	}
	return result, nil
}
