package service

import (
	"context"
	"fmt"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"
	passwordutil "buildadmin-go/internal/pkg/password"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

// UserService 承载会员管理的业务规则：账号唯一性、口令处理与归属
// （admin_id）解析。handler 只做参数绑定与响应返回。
type UserService struct {
	userM *adminmodel.UserRepository
}

func NewUserService(userM *adminmodel.UserRepository) *UserService {
	return &UserService{userM: userM}
}

// UserParams carries the plain (non-gin-bound) shape of a user create or
// update request. AdminID is nil when the request did not explicitly select
// an owner: creation then defaults to the acting administrator.
type UserParams struct {
	AdminID  *int32
	Username string
	Nickname string
	Email    string
	Mobile   string
	Avatar   string
	JoinIP   string
	JoinTime int64
	Password string
	Status   string
}

// Add runs the whole user-creation flow: username uniqueness, password
// hashing and the scoped transactional write.
func (s *UserService) Add(ctx context.Context, p UserParams, actor data_scope.Actor) error {
	usernameExists, err := s.userM.UsernameExists(ctx, p.Username)
	if err != nil {
		return err
	}
	if usernameExists {
		return cErr.BadRequest("Account exist")
	}
	if p.Password == "" {
		return cErr.BadRequest("Please input correct password")
	}

	var user model.User
	if err := copier.Copy(&user, p); err != nil {
		return err
	}
	if p.AdminID != nil {
		user.AdminID = *p.AdminID
	}
	hash, err := passwordutil.Hash(p.Password)
	if err != nil {
		return err
	}
	user.Password = hash

	return s.userM.AddWithActor(ctx, &user, actor)
}

// Edit runs the whole user-update flow: reload, owner resolution, password
// hashing (when provided) and the scoped transactional write.
func (s *UserService) Edit(ctx context.Context, id int32, p UserParams, actor data_scope.Actor) error {
	user, err := s.userM.GetOneWithActor(ctx, actor, id)
	if err != nil {
		return err
	}
	currentAdminID := user.AdminID

	if err := copier.Copy(&user, p); err != nil {
		return err
	}
	if p.AdminID != nil {
		user.AdminID = *p.AdminID
	} else {
		user.AdminID = currentAdminID
	}
	return s.userM.EditWithActor(ctx, &user, p.Password, actor)
}

// UpdateStatus validates and applies a scoped switch-style status update.
func (s *UserService) UpdateStatus(ctx context.Context, id int32, status string, actor data_scope.Actor) error {
	if err := ValidateAccountStatusValue(status); err != nil {
		return err
	}
	return s.userM.UpdateStatusWithActor(ctx, id, status, actor)
}

// Del runs the whole scoped user-deletion flow: id normalization, actor
// validation, the FOR UPDATE lock inside the actor's scope, the money-log
// orphan guard and the atomic scoped delete — all in one transaction.
func (s *UserService) Del(ctx context.Context, ids []int32, actor data_scope.Actor) error {
	normalized, err := normalizeIDs(ids, "user", true, func(id int32) error {
		return fmt.Errorf("invalid user id %d", id)
	})
	if err != nil {
		return err
	}
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	return s.userM.Transaction(ctx, func(tx *gorm.DB) error {
		if err := s.userM.LockUsersWithActor(ctx, tx, normalized, actor); err != nil {
			return err
		}
		// Reject deletion if any user still has money logs to prevent orphans.
		n, err := s.userM.CountMoneyLogsByUserIDs(tx, normalized)
		if err != nil {
			return err
		}
		if n > 0 {
			return cErr.BadRequest("user has money logs, cannot delete")
		}
		return s.userM.DeleteScopedWithActor(ctx, tx, normalized, actor)
	})
}
