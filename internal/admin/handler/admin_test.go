package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func ptr(v int32) *int32 { return &v }

func TestBuildAdminTreeOptions(t *testing.T) {
	admins := []*model.Admin{
		{ID: 1, Nickname: "Root", Username: "root_user"},
		{ID: 2, Nickname: "Child", Username: "child_user", ParentID: ptr(1)},
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
		username := o["username"].(string)
		if id == 1 && strings.Contains(text, "├") {
			t.Errorf("root option should not be prefixed: %s", text)
		}
		if id == 1 && username != "root_user(ID:1)" {
			t.Errorf("root username = %q", username)
		}
		if id == 2 && !strings.Contains(text, "Child(ID:2)") {
			t.Errorf("unexpected child label: %s", text)
		}
		if id == 2 && username != "child_user(ID:2)" {
			t.Errorf("child username = %q", username)
		}
	}
	if !seen[1] || !seen[2] {
		t.Fatal("missing options")
	}
}

func TestBuildFlatAdminOptions(t *testing.T) {
	admins := []*model.Admin{
		{ID: 3, Nickname: "A", Username: "alpha"},
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
	if opts[0]["username"].(string) != "alpha(ID:3)" {
		t.Fatalf("username = %s", opts[0]["username"])
	}
}

func TestSelectErrorIsHandled(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/admin/auth.Admin/index?select=1", nil)
	db, err := gorm.Open(handlerTestDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	h := NewAdminHandler(nil, adminmodel.NewAdminRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}), nil, nil)
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
