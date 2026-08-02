// Package money provides channel-neutral balance-change primitives shared by
// the admin and user-facing lanes (v3.0.0 stage D1).
//
// BalanceService owns the balance-changing transaction chain: FOR UPDATE row
// lock (under an explicit Scope), target-user ownership check, negative-balance
// rejection, atomic money update and the money-log write. It is transport
// neutral: it returns domain errors only and never HTTP-shaped errors; channel
// adapters (handlers/routers) are responsible for mapping ErrInsufficientBalance,
// ErrUserNotFound, ErrNoOwner and ErrScopeRequired to their lane's HTTP
// semantics. It never opens, commits or rolls back transactions: callers pass
// their own tx so the balance change is atomic with their business writes.
//
// Table names are resolved through the session NamingStrategy exactly like the
// rest of the application (SingularTable + TablePrefix), so the service needs
// no prefix configuration of its own.
package money

import (
	"errors"
	"fmt"

	model "go-build-admin/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Sentinel domain errors. Channel adapters map these to lane-specific HTTP
// responses; this package never returns transport-shaped errors.
var (
	// ErrInsufficientBalance is returned when the delta would drive the
	// target user's balance below zero.
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrUserNotFound is returned when no user matches ApplyInput.UserID. It
	// wraps gorm.ErrRecordNotFound so callers may keep matching the GORM
	// sentinel.
	ErrUserNotFound = fmt.Errorf("money: target user not found: %w", gorm.ErrRecordNotFound)
	// ErrNoOwner is returned when the target user has no owning admin
	// (admin_id == 0); a money log must always be attributed to an owner.
	ErrNoOwner = errors.New("target user has no owner")
	// ErrScopeRequired is returned when ApplyInput.Scope is nil. Authorization
	// is explicit: there is no implicit "nil means unrestricted" scope.
	ErrScopeRequired = errors.New("money: scope is required")
)

// Scope is a row-level authorization callback applied to every balance query
// (the FOR UPDATE read and the atomic update). Admin-lane adapters derive it
// from data_scope.Enforcer; the identity SystemScope must be chosen explicitly
// for system flows.
type Scope func(db *gorm.DB) *gorm.DB

// SystemScope returns the explicit unrestricted scope. It reproduces the
// legacy "nil scope" behavior but must now be chosen deliberately; only system
// flows / cron jobs should use it.
func SystemScope() Scope {
	return func(db *gorm.DB) *gorm.DB {
		return db
	}
}

// ApplyInput carries one balance change.
type ApplyInput struct {
	// UserID is the target user whose balance changes.
	UserID int32
	// Delta is the signed balance change: positive adds, negative subtracts.
	Delta float64
	// Memo is recorded on the money log entry.
	Memo string
	// Scope is required (ErrScopeRequired when nil).
	Scope Scope
}

// BalanceService applies balance changes atomically. It is stateless; the
// constructor exists for uniform DI wiring.
type BalanceService struct{}

// NewBalanceService returns a stateless balance service.
func NewBalanceService() *BalanceService {
	return &BalanceService{}
}

// ApplyDelta applies one balance change inside the caller's transaction. The
// caller supplies the tx so the balance change and the log write commit
// atomically with its own business writes.
//
// The created money log is returned so adapters can surface the new entry
// (e.g. its ID) without re-reading; its owner (AdminID) is the target user's
// owner, never an operator identity.
func (s *BalanceService) ApplyDelta(tx *gorm.DB, in ApplyInput) (*model.MoneyLog, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidDB
	}
	if in.Scope == nil {
		return nil, ErrScopeRequired
	}

	var user model.User
	if err := in.Scope(tx.Model(&model.User{})).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", in.UserID).
		Take(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: user id %d", ErrUserNotFound, in.UserID)
		}
		return nil, err
	}
	if user.AdminID == 0 {
		return nil, ErrNoOwner
	}

	before := user.Money
	after := before + in.Delta
	if after < 0 {
		return nil, ErrInsufficientBalance
	}

	res := in.Scope(tx.Model(&model.User{})).
		Where("id = ?", in.UserID).
		UpdateColumn("money", gorm.Expr("money + ?", in.Delta))
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, gorm.ErrRecordNotFound
	}

	log := &model.MoneyLog{
		AdminID: user.AdminID,
		UserID:  user.ID,
		Money:   in.Delta,
		Before:  before,
		After:   after,
		Memo:    in.Memo,
	}
	if err := tx.Create(log).Error; err != nil {
		return nil, err
	}
	return log, nil
}
