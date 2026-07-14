package data_scope

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func ptr[T any](v T) *T {
	return &v
}

// testDialector is a minimal in-memory dialector for unit tests. It never
// opens a network connection.
type testDialector struct{}

func (testDialector) Name() string                                   { return "test" }
func (testDialector) Initialize(*gorm.DB) error                      { return nil }
func (testDialector) Migrator(*gorm.DB) gorm.Migrator                { return nil }
func (testDialector) DataTypeOf(*schema.Field) string                { return "" }
func (testDialector) DefaultValueOf(*schema.Field) clause.Expression { return nil }
func (testDialector) BindVarTo(w clause.Writer, stmt *gorm.Statement, v interface{}) {
	_ = stmt
	_ = v
	w.WriteByte('?')
}
func (testDialector) QuoteTo(w clause.Writer, s string)              { w.WriteString("`" + s + "`") }
func (testDialector) Explain(sql string, vars ...interface{}) string { return sql }

// openTestDB returns a GORM DB backed by the test-only dialector.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(testDialector{}, &gorm.Config{DryRun: true})
	require.NoError(t, err)
	require.NotNil(t, db)
	return db
}

func TestModeConstants(t *testing.T) {
	assert.Equal(t, Mode("auto"), ModeAuto)
	assert.Equal(t, Mode("required"), ModeRequired)
	assert.Equal(t, Mode("none"), ModeNone)
}

func TestIdentifierValidation(t *testing.T) {
	valid := []string{"admin_id", "a", "_hidden", "table_alias", "col1", "AdminID"}
	for _, s := range valid {
		assert.NoError(t, ValidateIdentifier(s), "expected %q to be valid", s)
		assert.True(t, IsValidIdentifier(s), "expected %q to be valid", s)
	}

	invalid := []string{"", "admin id", "admin.id", "admin`id", "1admin", "admin-id", "admin\"id", "'x'"}
	for _, s := range invalid {
		assert.Error(t, ValidateIdentifier(s), "expected %q to be invalid", s)
		assert.False(t, IsValidIdentifier(s), "expected %q to be invalid", s)
		assert.True(t, errors.Is(ValidateIdentifier(s), ErrInvalidIdentifier))
	}
}

func TestValidateOwnerRef(t *testing.T) {
	assert.NoError(t, ValidateOwnerRef(OwnerRef{TableAlias: "t", Column: "admin_id"}))
	assert.ErrorIs(t, ValidateOwnerRef(OwnerRef{TableAlias: "", Column: "admin_id"}), ErrInvalidIdentifier)
	assert.ErrorIs(t, ValidateOwnerRef(OwnerRef{TableAlias: "t", Column: "t.admin_id"}), ErrInvalidIdentifier)
	assert.ErrorIs(t, ValidateOwnerRef(OwnerRef{TableAlias: "t;drop", Column: "admin_id"}), ErrInvalidIdentifier)
}

func TestActorValidation(t *testing.T) {
	_, err := NewActor(0)
	assert.ErrorIs(t, err, ErrInvalidActor)

	_, err = NewActor(-1)
	assert.ErrorIs(t, err, ErrInvalidActor)

	a, err := NewActor(7)
	require.NoError(t, err)
	assert.Equal(t, int32(7), a.AdminID)
	assert.False(t, a.Unrestricted)

	ua, err := NewUnrestrictedActor(9)
	require.NoError(t, err)
	assert.Equal(t, int32(9), ua.AdminID)
	assert.True(t, ua.Unrestricted)

	// Contract: AdminID == 0 without explicit unrestricted flag is invalid.
	// Oracle Gate: both restricted and unrestricted actors require positive AdminID.
	assert.ErrorIs(t, ValidateActor(Actor{AdminID: 0}), ErrInvalidActor)
	assert.ErrorIs(t, ValidateActor(Actor{AdminID: 0, Unrestricted: true}), ErrInvalidActor)
	assert.NoError(t, ValidateActor(Actor{AdminID: 1, Unrestricted: true}))
}

func TestActorContext(t *testing.T) {
	ctx := context.Background()
	_, ok := ActorFromContext(ctx)
	assert.False(t, ok)

	a, err := NewUnrestrictedActor(42)
	require.NoError(t, err)
	ctx = WithActor(ctx, a)

	got, ok := ActorFromContext(ctx)
	require.True(t, ok)
	assert.Equal(t, a, got)
}

func TestResolveConfig_NilConfigIsAuto(t *testing.T) {
	r, err := ResolveConfig(nil, true)
	require.NoError(t, err)
	assert.Equal(t, ModeAuto, r.Mode)
	assert.Equal(t, "admin_id", r.OwnerColumn)
	assert.Equal(t, "AdminID", r.OwnerGoField)
	assert.True(t, r.AssignOnCreate)
	assert.Equal(t, "auto:admin_id", r.Source)

	p := r.Policy()
	assert.Equal(t, ModeAuto, p.Mode)
	assert.Equal(t, "admin_id", p.OwnerColumn)
	assert.True(t, p.AssignOnCreate)
}

func TestResolveConfig_AutoWithoutAdminID(t *testing.T) {
	r, err := ResolveConfig(&Config{Mode: ModeAuto}, false)
	require.NoError(t, err)
	assert.Equal(t, ModeNone, r.Mode)
	assert.Empty(t, r.OwnerColumn)
	assert.Equal(t, "auto:none", r.Source)
}

func TestResolveConfig_Required(t *testing.T) {
	r, err := ResolveConfig(&Config{Mode: ModeRequired, OwnerColumn: "owner_admin_id"}, false)
	require.NoError(t, err)
	assert.Equal(t, ModeRequired, r.Mode)
	assert.Equal(t, "owner_admin_id", r.OwnerColumn)
	assert.Equal(t, "OwnerAdminID", r.OwnerGoField)
	assert.False(t, r.AssignOnCreate)
}

func TestResolveConfig_RequiredWithAssignOnCreate(t *testing.T) {
	r, err := ResolveConfig(&Config{Mode: ModeRequired, OwnerColumn: "owner_id", AssignOnCreate: ptr(true)}, false)
	require.NoError(t, err)
	assert.True(t, r.AssignOnCreate)
}

func TestResolveConfig_RequiredOwnerColumnValidation(t *testing.T) {
	_, err := ResolveConfig(&Config{Mode: ModeRequired, OwnerColumn: ""}, false)
	assert.ErrorIs(t, err, ErrInvalidOwnerColumn)

	_, err = ResolveConfig(&Config{Mode: ModeRequired, OwnerColumn: "owner.id"}, false)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidOwnerColumn) || errors.Is(err, ErrInvalidIdentifier))
}

func TestResolveConfig_RequiredCustomValidator(t *testing.T) {
	customErr := errors.New("column not found")
	_, err := ResolveConfigWithOptions(
		&Config{Mode: ModeRequired, OwnerColumn: "admin_id"},
		false,
		ResolveOptions{ValidateOwnerColumn: func(string) error { return customErr }},
	)
	assert.ErrorIs(t, err, customErr)
}

func TestResolveConfig_NoneWithAdminIDRequiresOverride(t *testing.T) {
	_, err := ResolveConfig(&Config{Mode: ModeNone}, true)
	assert.ErrorIs(t, err, ErrInvalidMode)

	r, err := ResolveConfigWithOptions(&Config{Mode: ModeNone}, true, ResolveOptions{AllowNoneWithAdminID: true})
	require.NoError(t, err)
	assert.Equal(t, ModeNone, r.Mode)
}

func TestResolveConfig_InvalidMode(t *testing.T) {
	_, err := ResolveConfig(&Config{Mode: "bad"}, true)
	assert.ErrorIs(t, err, ErrInvalidMode)
}

func TestResolveConfig_AutoAdminIDAlwaysAssignOnCreate(t *testing.T) {
	r, err := ResolveConfig(&Config{Mode: ModeAuto, AssignOnCreate: ptr(true)}, true)
	require.NoError(t, err)
	assert.Equal(t, ModeAuto, r.Mode)
	assert.True(t, r.AssignOnCreate)

	_, err = ResolveConfig(&Config{Mode: ModeAuto, AssignOnCreate: ptr(false)}, true)
	assert.ErrorIs(t, err, ErrInvalidConfig)

	// nil still defaults to true.
	r, err = ResolveConfig(&Config{Mode: ModeAuto}, true)
	require.NoError(t, err)
	assert.True(t, r.AssignOnCreate)
}

func TestResolveConfig_AdminIDSpecialCase(t *testing.T) {
	// Auto must not infer admin.id from OwnerColumn; no admin_id => ModeNone.
	r, err := ResolveConfig(&Config{Mode: ModeAuto, OwnerColumn: "id"}, false)
	require.NoError(t, err)
	assert.Equal(t, ModeNone, r.Mode)
	assert.Empty(t, r.OwnerColumn)

	// admin.id must be explicit required.
	r, err = ResolveConfig(&Config{Mode: ModeRequired, OwnerColumn: "id"}, false)
	require.NoError(t, err)
	assert.Equal(t, ModeRequired, r.Mode)
	assert.Equal(t, "id", r.OwnerColumn)
	assert.False(t, r.AssignOnCreate)

	// required + "id" + assign-on-create=true is invalid.
	_, err = ResolveConfig(&Config{Mode: ModeRequired, OwnerColumn: "id", AssignOnCreate: ptr(true)}, false)
	assert.ErrorIs(t, err, ErrInvalidConfig)

	// required + "id" + nil also resolves to false.
	r, err = ResolveConfig(&Config{Mode: ModeRequired, OwnerColumn: "id", AssignOnCreate: nil}, false)
	require.NoError(t, err)
	assert.False(t, r.AssignOnCreate)
}

func TestDenyAllEnforcer_ActorMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	enf := NewDenyAllEnforcer()
	_, err := enf.Actor(ctx)
	assert.ErrorIs(t, err, ErrInvalidActor)
}

func TestClosureEnforcerPrefixAndSQLShape(t *testing.T) {
	for _, tc := range []struct{ prefix, table string }{{"", "admin_closure"}, {"ba_", "ba_admin_closure"}} {
		e := NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: tc.prefix}})
		assert.Equal(t, tc.table, e.ClosureTable())
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		a, err := NewActor(7)
		require.NoError(t, err)
		ctx.Set(actorContextKey, a)
		scoped := e.Scope(ctx, openTestDB(t).Table("resource"), OwnerRef{TableAlias: "resource", Column: "owner_admin_id"})
		where := fmt.Sprintf("%v", scoped.Statement.Clauses["WHERE"].Expression)
		assert.Contains(t, where, "EXISTS (SELECT 1 FROM `"+tc.table+"` AS closure")
		assert.Contains(t, where, "closure.descendant_id = `resource`.`owner_admin_id`")
	}
}

func TestClosureEnforcerInvalidPrefixesFailClosed(t *testing.T) {
	for _, prefix := range []string{"bad.prefix", "bad`prefix", "bad prefix"} {
		e := NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: prefix}})
		assert.Empty(t, e.ClosureTable())
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		a, _ := NewActor(7)
		ctx.Set(actorContextKey, a)
		assert.ErrorIs(t, e.Scope(ctx, openTestDB(t), OwnerRef{TableAlias: "resource", Column: "id"}).Error, ErrScopedAccessDenied)
	}
}

func TestDenyAllEnforcer_ActorFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	a, err := NewUnrestrictedActor(1)
	require.NoError(t, err)
	ctx.Set(actorContextKey, a)

	enf := NewDenyAllEnforcer()
	got, err := enf.Actor(ctx)
	require.NoError(t, err)
	assert.Equal(t, a, got)
}

func TestDenyAllEnforcer_ScopeUnrestrictedBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	a, err := NewUnrestrictedActor(1)
	require.NoError(t, err)
	ctx.Set(actorContextKey, a)

	db := openTestDB(t)
	enf := NewDenyAllEnforcer()
	scoped := enf.Scope(ctx, db, OwnerRef{TableAlias: "t", Column: "admin_id"})
	require.NotNil(t, scoped)
	assert.Equal(t, db, scoped, "unrestricted actor must receive the unmodified DB")
	assert.NoError(t, db.Error, "unrestricted path must not pollute the original DB")
}

func TestDenyAllEnforcer_ScopeRestrictedDenies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	a, err := NewActor(5)
	require.NoError(t, err)
	ctx.Set(actorContextKey, a)

	db := openTestDB(t)
	enf := NewDenyAllEnforcer()
	scoped := enf.Scope(ctx, db, OwnerRef{TableAlias: "t", Column: "admin_id"})
	require.NotNil(t, scoped)
	assert.NotEqual(t, db, scoped, "restricted actor must receive a derived DB")
	assert.ErrorIs(t, scoped.Error, ErrScopedAccessDenied)
	assert.NoError(t, db.Error, "original DB must not be polluted")
}

func TestDenyAllEnforcer_ScopeInvalidOwnerRefDenies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	a, err := NewActor(5)
	require.NoError(t, err)
	ctx.Set(actorContextKey, a)

	db := openTestDB(t)
	enf := NewDenyAllEnforcer()
	scoped := enf.Scope(ctx, db, OwnerRef{TableAlias: "t", Column: "t.admin_id"})
	require.NotNil(t, scoped)
	assert.NotEqual(t, db, scoped, "invalid owner ref must receive a derived DB")
	assert.ErrorIs(t, scoped.Error, ErrInvalidIdentifier)
	assert.ErrorIs(t, scoped.Error, ErrScopedAccessDenied)
	assert.NoError(t, db.Error, "original DB must not be polluted")
}

func TestDenyAllEnforcer_ScopeInvalidActorDenies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	// No actor attached.
	db := openTestDB(t)
	enf := NewDenyAllEnforcer()
	scoped := enf.Scope(ctx, db, OwnerRef{TableAlias: "t", Column: "admin_id"})
	require.NotNil(t, scoped)
	assert.NotEqual(t, db, scoped, "invalid actor must receive a derived DB")
	assert.ErrorIs(t, scoped.Error, ErrInvalidActor)
	assert.ErrorIs(t, scoped.Error, ErrScopedAccessDenied)
	assert.NoError(t, db.Error, "original DB must not be polluted")
}

func TestDenyAllEnforcer_ScopeNilDB(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	a, err := NewActor(5)
	require.NoError(t, err)
	ctx.Set(actorContextKey, a)

	enf := NewDenyAllEnforcer()
	assert.Nil(t, enf.Scope(ctx, nil, OwnerRef{TableAlias: "t", Column: "admin_id"}))
}

func TestStubHierarchyWriter(t *testing.T) {
	w := NewStubHierarchyWriter()
	assert.ErrorIs(t, w.LinkNewNode(context.Background(), nil, 1, nil), ErrNotImplemented)
	assert.ErrorIs(t, w.MoveSubtree(context.Background(), nil, 1, nil), ErrNotImplemented)
}
