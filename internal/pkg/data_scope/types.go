// Package data_scope defines the frozen contracts and fail-closed scaffolding
// for hierarchical row authorization. The package intentionally does not own
// transaction lifecycle, authentication middleware, or table prefix resolution;
// those responsibilities belong to callers and future runtime lanes.
package data_scope

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Mode selects the data-scope resolution strategy.
type Mode string

const (
	// ModeAuto detects the owner convention. When the resource has an exact
	// admin_id column it resolves to hierarchical scope; otherwise it resolves
	// to explicit global policy (ModeNone).
	ModeAuto Mode = "auto"
	// ModeRequired enforces an explicitly configured owner column.
	ModeRequired Mode = "required"
	// ModeNone disables row-level scope for the resource.
	ModeNone Mode = "none"
)

// Config is the persisted generator/user input. A nil Config resolves to
// ModeAuto, preserving legacy behavior.
type Config struct {
	Mode            Mode     `json:"mode"`
	OwnerColumn     string   `json:"ownerColumn,omitempty"`
	ReadExtraOwners []string `json:"readExtraOwners,omitempty"`
	AssignOnCreate  *bool    `json:"assignOnCreate,omitempty"`
	// Reassignable permits editing the owner column: restricted operators may
	// only reassign to an admin inside their own subtree (self + descendants),
	// unrestricted operators may pick any existing admin. Only meaningful when
	// an owner column is resolved.
	Reassignable bool `json:"reassignable,omitempty"`
	// InheritFrom makes Add inherit this resource's owner from a parent
	// entity instead of assigning the operating admin. The named table is
	// looked up by the join column and its admin_id is copied. Mutually
	// exclusive with Reassignable; only meaningful when an owner column is
	// resolved. Child tables are registered on the parent repository's
	// CascadeOwners() anchor block automatically at generation time.
	InheritFrom *InheritRef `json:"inheritFrom,omitempty"`
}

// CascadeOwner names one child table whose redundant owner column is kept in
// sync with the parent entity. It is the element type of the generated parent
// repository's CascadeOwners() registry (maintained by the CRUD generator
// anchor block) and is also read by the cascade:sync reconciliation CLI.
type CascadeOwner struct {
	Table       string `json:"table"`
	ByColumn    string `json:"byColumn"`
	OwnerColumn string `json:"ownerColumn,omitempty"` // 空时归一为 admin_id
}

// InheritRef names the parent entity a child table inherits its owner from on
// Add. ByColumn is the child's own column referencing the parent's primary
// key (for example user_id on an order table).
type InheritRef struct {
	Table    string `json:"table"`
	ByColumn string `json:"byColumn"`
}

// Resolved is the effective policy after applying conventions and defaults.
type Resolved struct {
	Mode            Mode
	OwnerColumn     string
	OwnerGoField    string
	ReadExtraOwners []string
	AssignOnCreate  bool
	Reassignable    bool
	InheritFrom     *InheritRef
	Source          string
}

// ResourcePolicy is the compact runtime policy derived from Resolved.
type ResourcePolicy struct {
	Mode            Mode
	OwnerColumn     string
	ReadExtraOwners []string
	AssignOnCreate  bool
	Reassignable    bool
	InheritFrom     *InheritRef
}

// Actor carries the authenticated administrator identity and an explicit
// unrestricted flag. Only a typed actor with Unrestricted == true bypasses
// scope; all other access is scoped and, in this scaffold, denied.
type Actor struct {
	AdminID      int32
	Unrestricted bool
}

// OwnerRef names the static, validated table alias and column used by
// a scoped query. Both fields must be plain identifiers; SQL expressions are
// rejected.
type OwnerRef struct {
	TableAlias string
	Column     string
}

// Enforcer extracts the actor for a request and applies a scoped base query.
// Real closure-SQL implementations belong to a future runtime lane; the
// scaffold provided here is fail-closed.
type Enforcer interface {
	Actor(ctx *gin.Context) (Actor, error)
	Scope(ctx *gin.Context, db *gorm.DB, owner OwnerRef) *gorm.DB
}

// ReadScopeEnforcer is an optional capability for read queries that may be
// visible through additional owner columns. It deliberately does not extend
// Enforcer so existing implementations retain their contract.
type ReadScopeEnforcer interface {
	ScopeWithExtraOwners(ctx *gin.Context, db *gorm.DB, primary OwnerRef, extras []OwnerRef) *gorm.DB
}

// ScopeRead applies a dual-owner read scope when supported. With no extra
// owners it calls Scope directly, preserving the legacy query path exactly.
func ScopeRead(ctx *gin.Context, db *gorm.DB, enforcer Enforcer, primary OwnerRef, extras []OwnerRef) *gorm.DB {
	if len(extras) == 0 {
		return enforcer.Scope(ctx, db, primary)
	}
	capability, ok := enforcer.(ReadScopeEnforcer)
	if !ok {
		return addScopeError(db, ErrScopedAccessDenied)
	}
	return capability.ScopeWithExtraOwners(ctx, db, primary, extras)
}

// HierarchyWriter maintains the admin_closure read model. Implementations
// receive a transaction from the caller and must not commit or roll back.
type HierarchyWriter interface {
	LinkNewNode(ctx context.Context, tx *gorm.DB, nodeID int32, parentID *int32) error
	MoveSubtree(ctx context.Context, tx *gorm.DB, nodeID int32, newParentID *int32) error
}

// Sentinel errors. They are returned by helpers and may be inspected with
// errors.Is in callers and tests.
var (
	ErrInvalidMode        = errors.New("data_scope: invalid mode")
	ErrInvalidOwnerColumn = errors.New("data_scope: invalid owner column")
	ErrInvalidActor       = errors.New("data_scope: invalid actor")
	ErrInvalidConfig      = errors.New("data_scope: invalid config")
	ErrInvalidIdentifier  = errors.New("data_scope: invalid identifier")
	ErrScopedAccessDenied = errors.New("data_scope: scoped access denied")
	ErrNotImplemented     = errors.New("data_scope: not implemented")
)

// identifierRE matches plain SQL identifiers: letters, digits, and
// underscores, with a letter or underscore as the first character.
var identifierRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// IsValidIdentifier reports whether s is a safe static identifier.
// It rejects empty strings, whitespace, dots, quotes, and any other
// characters that could be interpreted as SQL syntax.
func IsValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	return identifierRE.MatchString(s)
}

// ValidateIdentifier returns ErrInvalidIdentifier unless s is a safe static
// identifier.
func ValidateIdentifier(s string) error {
	if !IsValidIdentifier(s) {
		return fmt.Errorf("%w: %q", ErrInvalidIdentifier, s)
	}
	return nil
}

// ValidateOwnerRef returns ErrInvalidIdentifier if either the table alias or
// the column is not a safe static identifier. Fail-closed callers should treat
// the returned error as a denial condition.
func ValidateOwnerRef(ref OwnerRef) error {
	if err := ValidateIdentifier(ref.TableAlias); err != nil {
		return err
	}
	if err := ValidateIdentifier(ref.Column); err != nil {
		return err
	}
	return nil
}

// ValidateActor rejects obviously invalid actors. A restricted actor must have
// a positive AdminID. Both restricted and unrestricted actors must have a
// positive AdminID; Unrestricted must be explicit. Zero or negative AdminID
// is invalid and fail-closed.
func ValidateActor(a Actor) error {
	if a.AdminID <= 0 {
		return fmt.Errorf("%w: adminID must be positive, got %d", ErrInvalidActor, a.AdminID)
	}
	return nil
}

// NewActor returns a restricted actor. AdminID must be positive.
func NewActor(adminID int32) (Actor, error) {
	a := Actor{AdminID: adminID}
	if err := ValidateActor(a); err != nil {
		return Actor{}, err
	}
	return a, nil
}

// NewUnrestrictedActor returns an actor that explicitly bypasses scope.
// AdminID must be positive.
func NewUnrestrictedActor(adminID int32) (Actor, error) {
	a := Actor{AdminID: adminID, Unrestricted: true}
	if err := ValidateActor(a); err != nil {
		return Actor{}, err
	}
	return a, nil
}

// actorContextKey is intentionally a string so that it works both with
// context.WithValue and with gin.Context.Get/Set lookups.
const ActorContextKey = "buildadmin-go/internal/pkg/data_scope.actor"
const actorContextKey = ActorContextKey

// WithActor attaches an Actor to a context.
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey, actor)
}

// ActorFromContext retrieves a previously attached Actor.
func ActorFromContext(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorContextKey).(Actor)
	return a, ok
}

// Policy returns the compact runtime policy for a resolved configuration.
func (r Resolved) Policy() ResourcePolicy {
	return ResourcePolicy{
		Mode:            r.Mode,
		OwnerColumn:     r.OwnerColumn,
		ReadExtraOwners: append([]string(nil), r.ReadExtraOwners...),
		AssignOnCreate:  r.AssignOnCreate,
		Reassignable:    r.Reassignable,
		InheritFrom:     r.InheritFrom,
	}
}

// commonInitialisms maps lowercase identifier fragments to canonical Go
// initialisms so that "admin_id" resolves to "AdminID".
var commonInitialisms = map[string]string{
	"id":  "ID",
	"ids": "IDs",
	"uid": "UID",
	"url": "URL",
}

// deriveGoField converts a snake_case column name to a Go struct field name.
// It is used by the resolver when the Go field is not explicitly supplied.
func deriveGoField(column string) string {
	parts := strings.Split(column, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if canon, ok := commonInitialisms[p]; ok {
			b.WriteString(canon)
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}
