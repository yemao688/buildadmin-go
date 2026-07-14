package data_scope

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"go-build-admin/conf"
	"gorm.io/gorm"
)

type ClosureEnforcer struct{ closureTable string }

// ClosureTable reports the resolved closure table for diagnostics and tests.
func (e *ClosureEnforcer) ClosureTable() string {
	if e == nil {
		return ""
	}
	return e.closureTable
}

// Deprecated: retained only for compatibility with pre-runtime tests. It is
// not used by production or Wire provider paths.
type DenyAllEnforcer = ClosureEnforcer

// Deprecated: use NewClosureEnforcer.
func NewDenyAllEnforcer() *DenyAllEnforcer { return &ClosureEnforcer{} }

func NewClosureEnforcer(config *conf.Configuration) *ClosureEnforcer {
	e := &ClosureEnforcer{}
	prefix := ""
	if config != nil {
		prefix = config.Database.Prefix
	}
	if ValidateTablePrefix(prefix) == nil {
		table := prefix + "admin_closure"
		if ValidateIdentifier(table) == nil {
			e.closureTable = table
		}
	}
	return e
}

// Actor extracts a typed Actor from the request context. If no actor is
// attached, or if the attached actor fails validation, the call fails closed.
func (ClosureEnforcer) Actor(ctx *gin.Context) (Actor, error) {
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
func (e ClosureEnforcer) Scope(ctx *gin.Context, db *gorm.DB, owner OwnerRef) *gorm.DB {
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

	if e.closureTable == "" {
		return addScopeError(db, fmt.Errorf("%w: closure table is not configured", ErrScopedAccessDenied))
	}
	closure := quoteIdentifier(e.closureTable)
	condition := fmt.Sprintf("EXISTS (SELECT 1 FROM %s AS self_closure WHERE self_closure.ancestor_id = ? AND self_closure.descendant_id = ?) AND EXISTS (SELECT 1 FROM %s AS closure WHERE closure.ancestor_id = ? AND closure.descendant_id = %s.%s)", closure, closure, quoteIdentifier(owner.TableAlias), quoteIdentifier(owner.Column))
	return db.Session(&gorm.Session{}).Where(condition, actor.AdminID, actor.AdminID, actor.AdminID)
}

func quoteIdentifier(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }

func SetActor(ctx *gin.Context, actor Actor) error {
	if ctx == nil {
		return fmt.Errorf("%w: nil context", ErrInvalidActor)
	}
	if err := ValidateActor(actor); err != nil {
		return err
	}
	ctx.Set(actorContextKey, actor)
	return nil
}

// addScopeError creates an independent GORM session, adds the supplied errors,
// and returns it. The original db is left untouched so that a shared session
// cannot be poisoned by one failed scope application.
func addScopeError(db *gorm.DB, errs ...error) *gorm.DB {
	tx := db.Session(&gorm.Session{})
	_ = tx.AddError(errors.Join(errs...))
	return tx
}
