package service

import (
	"context"
	"strconv"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	passwordutil "buildadmin-go/internal/pkg/password"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

// AdminService 承载管理员管理的业务规则：口令处理、分组/规则装配、
// 父级解析、状态流转与删除编排。handler 只做参数绑定与响应返回。
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

// Add runs the whole administrator-creation flow: group authorization,
// parent resolution, password hashing and the transactional write.
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
		if err := s.adminM.CheckParentInScopeWithActor(ctx, actor, *parentID); err != nil {
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

	return s.adminM.AddWithActor(ctx, admin, p.GroupArr, actor)
}

// Edit runs the whole administrator-update flow: reload, self-disable guard,
// parent resolution, group delta authorization, password hashing and the
// transactional write.
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
		if err := s.adminM.CheckParentInScopeWithActor(ctx, actor, *parentID); err != nil {
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

	return s.adminM.EditWithActor(ctx, admin, changed, parentID, omit, p.GroupArr, actor)
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
// actor validation and the hierarchy delete (scope re-verification,
// subordinate rejection, closure cleanup) in one transaction.
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
	enforcer := data_scope.NewClosureEnforcer(s.config)
	return s.adminM.Transaction(ctx, func(tx *gorm.DB) error {
		return adminmodel.NewAdminHierarchy(s.config).DeleteAdmins(ctx, tx, idList, actor, enforcer)
	})
}
