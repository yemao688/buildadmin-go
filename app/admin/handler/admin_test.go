package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"go-build-admin/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func ptr(v int32) *int32 { return &v }

func TestResolveParentIDForAdd(t *testing.T) {
	restricted := data_scope.Actor{AdminID: 5}
	unrestricted := data_scope.Actor{AdminID: 1, Unrestricted: true}

	cases := []struct {
		name    string
		p       NullableParentID
		actor   data_scope.Actor
		want    *int32
		wantErr bool
	}{
		{"restricted omitted defaults to actor", NullableParentID{}, restricted, ptr(5), false},
		{"restricted null defaults to actor", NullableParentID{IsSet: true}, restricted, ptr(5), false},
		{"restricted 0 defaults to actor", NullableParentID{IsSet: true, Value: ptr(0)}, restricted, ptr(5), false},
		{"unrestricted omitted is root", NullableParentID{}, unrestricted, nil, false},
		{"unrestricted 0 is root", NullableParentID{IsSet: true, Value: ptr(0)}, unrestricted, nil, false},
		{"explicit positive kept", NullableParentID{IsSet: true, Value: ptr(7)}, restricted, ptr(7), false},
		{"negative rejected", NullableParentID{IsSet: true, Value: ptr(-1)}, restricted, nil, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveParentIDForAdd(tc.p, tc.actor)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !int32PtrEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveParentIDForEdit(t *testing.T) {
	restricted := data_scope.Actor{AdminID: 5}
	unrestricted := data_scope.Actor{AdminID: 1, Unrestricted: true}

	cases := []struct {
		name        string
		p           NullableParentID
		current     *int32
		actor       data_scope.Actor
		want        *int32
		wantChanged bool
		wantErr     bool
	}{
		{"omitted unchanged", NullableParentID{}, ptr(3), restricted, ptr(3), false, false},
		{"null unchanged", NullableParentID{IsSet: true}, ptr(3), restricted, ptr(3), false, false},
		{"restricted cannot root", NullableParentID{IsSet: true, Value: ptr(0)}, ptr(3), restricted, nil, false, true},
		{"unrestricted can root", NullableParentID{IsSet: true, Value: ptr(0)}, ptr(3), unrestricted, nil, true, false},
		{"unrestricted root no-op", NullableParentID{IsSet: true, Value: ptr(0)}, nil, unrestricted, nil, false, false},
		{"positive changed", NullableParentID{IsSet: true, Value: ptr(7)}, ptr(3), restricted, ptr(7), true, false},
		{"positive unchanged", NullableParentID{IsSet: true, Value: ptr(3)}, ptr(3), restricted, ptr(3), false, false},
		{"negative rejected", NullableParentID{IsSet: true, Value: ptr(-1)}, ptr(3), restricted, nil, false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed, err := resolveParentIDForEdit(tc.p, tc.current, tc.actor)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !int32PtrEqual(got, tc.want) {
				t.Fatalf("parent got %v, want %v", got, tc.want)
			}
			if changed != tc.wantChanged {
				t.Fatalf("changed=%v, want %v", changed, tc.wantChanged)
			}
		})
	}
}

func TestIsMovingUnderSelf(t *testing.T) {
	if !isMovingUnderSelf(10, ptr(10)) {
		t.Error("moving node 10 under itself should be detected")
	}
	if isMovingUnderSelf(10, nil) {
		t.Error("nil parent is not a self-move")
	}
	if isMovingUnderSelf(10, ptr(5)) {
		t.Error("different parent is not a self-move")
	}
	// New administrator has no ID yet; creating under the current actor is valid.
	if isMovingUnderSelf(0, ptr(7)) {
		t.Error("new admin under actor 7 must not be treated as self-move")
	}
}

func TestBuildAdminTreeOptions(t *testing.T) {
	admins := []*model.Admin{
		{ID: 1, Nickname: "Root"},
		{ID: 2, Nickname: "Child", ParentID: ptr(1)},
	}

	opts := buildAdminTreeOptions(admins)
	if len(opts) != 2 {
		t.Fatalf("len = %d, want 2", len(opts))
	}

	seen := map[int]bool{}
	for _, o := range opts {
		id := o["id"].(int)
		seen[id] = true
		text := o["nickname"].(string)
		if id == 1 && strings.Contains(text, "├") {
			t.Errorf("root option should not be prefixed: %s", text)
		}
		if id == 2 && !strings.Contains(text, "Child(ID:2)") {
			t.Errorf("unexpected child label: %s", text)
		}
	}
	if !seen[1] || !seen[2] {
		t.Fatal("missing options")
	}
}

func TestBuildFlatAdminOptions(t *testing.T) {
	admins := []*model.Admin{
		{ID: 3, Nickname: "A"},
	}
	opts := buildFlatAdminOptions(admins)
	if len(opts) != 1 {
		t.Fatalf("len = %d, want 1", len(opts))
	}
	if opts[0]["id"].(int32) != 3 {
		t.Fatalf("id = %v", opts[0]["id"])
	}
	if opts[0]["nickname"].(string) != "A(ID:3)" {
		t.Fatalf("nickname = %s", opts[0]["nickname"])
	}
}

func TestSetAdminPasswordHashesAfterCopy(t *testing.T) {
	admin := model.Admin{Password: "plaintext"}
	setAdminPassword(&admin, "correct horse battery staple")
	if admin.Password == "correct horse battery staple" || admin.Password == "" {
		t.Fatalf("password was not hashed: %q", admin.Password)
	}
	if admin.Salt == "" || admin.Password != utils.EncryptPassword("correct horse battery staple", admin.Salt) {
		t.Fatal("password hash cannot be verified with the generated salt")
	}
}

func TestSelectErrorIsHandled(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/admin/auth.Admin/index?select=1", nil)
	db, err := gorm.Open(handlerTestDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	h := NewAdminHandler(nil, model.NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}), nil)
	data, matched, err := h.Select(ctx)
	if !matched || err == nil || data != nil {
		t.Fatalf("Select error result = (%v, %v, %v), want handled error", data, matched, err)
	}
}

type handlerTestDialector struct{}

func (handlerTestDialector) Name() string                                   { return "handler-test" }
func (handlerTestDialector) Initialize(*gorm.DB) error                      { return nil }
func (handlerTestDialector) Migrator(*gorm.DB) gorm.Migrator                { return nil }
func (handlerTestDialector) DataTypeOf(*schema.Field) string                { return "" }
func (handlerTestDialector) DefaultValueOf(*schema.Field) clause.Expression { return nil }
func (handlerTestDialector) BindVarTo(w clause.Writer, _ *gorm.Statement, _ interface{}) {
	w.WriteByte('?')
}
func (handlerTestDialector) QuoteTo(w clause.Writer, s string)           { w.WriteString("`" + s + "`") }
func (handlerTestDialector) Explain(sql string, _ ...interface{}) string { return sql }
