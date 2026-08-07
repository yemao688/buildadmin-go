package data_scope

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"buildadmin-go/internal/conf"
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
	return e.ScopeWithExtraOwners(ctx, db, owner, nil)
}

// ScopeWithExtraOwners scopes rows owned by the primary owner or any extra
// owner column. Every owner reference is validated before SQL is constructed.
func (e ClosureEnforcer) ScopeWithExtraOwners(ctx *gin.Context, db *gorm.DB, primary OwnerRef, extras []OwnerRef) *gorm.DB {
	actor, err := e.Actor(ctx)
	if err != nil {
		return addScopeError(db, err, ErrScopedAccessDenied)
	}
	return e.scopeOwners(ctx, db, actor, append([]OwnerRef{primary}, extras...))
}

// ScopeWithActor applies the closure scope for an explicitly supplied actor.
// It is the transport-free counterpart of Scope for service/domain layers
// that receive the actor as a parameter instead of a gin request context.
func (e ClosureEnforcer) ScopeWithActor(ctx context.Context, db *gorm.DB, actor Actor, owner OwnerRef) *gorm.DB {
	return e.scopeOwners(ctx, db, actor, []OwnerRef{owner})
}

func (e ClosureEnforcer) scopeOwners(ctx context.Context, db *gorm.DB, actor Actor, refs []OwnerRef) *gorm.DB {
	if db == nil {
		// The interface does not allow returning an error. Return nil so the
		// caller panics deterministically rather than silently running
		// unscoped.
		return nil
	}

	if err := ValidateActor(actor); err != nil {
		return addScopeError(db, err, ErrScopedAccessDenied)
	}

	for _, ref := range refs {
		if err := ValidateOwnerRef(ref); err != nil {
			return addScopeError(db, err, ErrScopedAccessDenied)
		}
	}

	if actor.Unrestricted {
		return db
	}

	if e.closureTable == "" {
		return addScopeError(db, fmt.Errorf("%w: closure table is not configured", ErrScopedAccessDenied))
	}
	closure := quoteIdentifier(e.closureTable)

	// The self-EXISTS guard (the actor must have its mandatory closure
	// self-row) is constant for the whole request, so the actor construction
	// lane verifies it once and caches the result per request. When verified
	// present, the per-query self-EXISTS subquery is dropped and only the
	// ownership branches remain. When verified absent — or when the request
	// never performed the verification — the full condition is kept, so
	// denial semantics are byte-identical to the pre-cache behavior.
	skipSelfCheck := false
	if checked, ok := SelfRowChecked(ctx); ok {
		skipSelfCheck = checked
	}

	branches := make([]string, len(refs))
	args := make([]interface{}, 0, len(refs)+2)
	for i, ref := range refs {
		branches[i] = fmt.Sprintf("EXISTS (SELECT 1 FROM %s AS closure WHERE closure.ancestor_id = ? AND closure.descendant_id = %s.%s)", closure, quoteIdentifier(ref.TableAlias), quoteIdentifier(ref.Column))
		args = append(args, actor.AdminID)
	}
	if skipSelfCheck {
		return db.Session(&gorm.Session{}).Where("("+strings.Join(branches, " OR ")+")", args...)
	}
	args = append([]interface{}{actor.AdminID, actor.AdminID}, args...)
	return db.Session(&gorm.Session{}).Where(
		fmt.Sprintf("EXISTS (SELECT 1 FROM %s AS self_closure WHERE self_closure.ancestor_id = ? AND self_closure.descendant_id = ?) AND (%s)", closure, strings.Join(branches, " OR ")),
		args...,
	)
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
