package data_scope

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DenyAllEnforcer is a fail-closed Enforcer scaffold. It provides typed actor
// extraction from the request context and applies a deny-all scope to any
// non-unrestricted actor. It does not implement the real closure-SQL predicate;
// that work belongs to the runtime lane.
type DenyAllEnforcer struct{}

// NewDenyAllEnforcer returns the shared fail-closed scaffold.
func NewDenyAllEnforcer() *DenyAllEnforcer {
	return &DenyAllEnforcer{}
}

// Actor extracts a typed Actor from the request context. If no actor is
// attached, or if the attached actor fails validation, the call fails closed.
func (DenyAllEnforcer) Actor(ctx *gin.Context) (Actor, error) {
	if ctx == nil {
		return Actor{}, fmt.Errorf("%w: nil context", ErrInvalidActor)
	}
	a, ok := ActorFromContext(ctx)
	if !ok {
		return Actor{}, fmt.Errorf("%w: actor not present in context", ErrInvalidActor)
	}
	if err := ValidateActor(a); err != nil {
		return Actor{}, err
	}
	return a, nil
}

// Scope applies a scoped base query. It is fail-closed: any problem
// extracting the actor, any invalid identifier, or any non-unrestricted actor
// receives a derived GORM session carrying the original cause and
// ErrScopedAccessDenied. Only an explicit unrestricted actor receives the
// original DB. The returned DB is never the original input for scoped
// (non-bypass) requests, and the original shared DB is never mutated.
func (e DenyAllEnforcer) Scope(ctx *gin.Context, db *gorm.DB, owner OwnerRef) *gorm.DB {
	if db == nil {
		// The interface does not allow returning an error. Return nil so the
		// caller panics deterministically rather than silently running
		// unscoped.
		return nil
	}

	actor, err := e.Actor(ctx)
	if err != nil {
		return addScopeError(db, err, ErrScopedAccessDenied)
	}

	if err := ValidateOwnerRef(owner); err != nil {
		return addScopeError(db, err, ErrScopedAccessDenied)
	}

	if actor.Unrestricted {
		return db
	}

	// Fail closed: block the scoped query until the real closure predicate
	// is integrated in the runtime lane.
	return addScopeError(db, fmt.Errorf("%w: actor %d is restricted", ErrScopedAccessDenied, actor.AdminID))
}

// addScopeError creates an independent GORM session, adds the supplied errors,
// and returns it. The original db is left untouched so that a shared session
// cannot be poisoned by one failed scope application.
func addScopeError(db *gorm.DB, errs ...error) *gorm.DB {
	tx := db.Session(&gorm.Session{})
	_ = tx.AddError(errors.Join(errs...))
	return tx
}
