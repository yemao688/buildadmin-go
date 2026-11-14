package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/tree"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func ptr(v int32) *int32 { return &v }

// mustType 断言测试数据的动态类型，不符即终止用例。
func mustType[T any](t *testing.T, v any) T {
	t.Helper()
	typed, ok := v.(T)
	if !ok {
		t.Fatalf("unexpected type %T, want %T", v, *new(T))
	}
	return typed
}

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
		id := mustType[int](t, o["id"])
		seen[id] = true
		text := mustType[string](t, o["nickname"])
		username := mustType[string](t, o["username"])
		if id == 1 && strings.Contains(text, "├") {
			t.Errorf("root option should not be prefixed: %s", text)
		}
		if id == 1 && text != "Root(ID:1)" {
			t.Errorf("root nickname = %q, want un-prefixed label", text)
		}
		if id == 1 && username != "root_user(ID:1)" {
			t.Errorf("root username = %q", username)
		}
		if id == 2 && text != "Child(ID:2)" {
			t.Errorf("child nickname = %q, want un-prefixed label", text)
		}
		if id == 2 && !strings.Contains(username, "child_user(ID:2)") {
			t.Errorf("child username = %q, want tree-prefixed label", username)
		}
		if id == 2 && !strings.Contains(username, "└") {
			t.Errorf("child username = %q, want tree indent marker", username)
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
	if mustType[int32](t, opts[0]["id"]) != 3 {
		t.Fatalf("id = %v", opts[0]["id"])
	}
	if mustType[string](t, opts[0]["nickname"]) != "A(ID:3)" {
		t.Fatalf("nickname = %s", opts[0]["nickname"])
	}
	if mustType[string](t, opts[0]["username"]) != "alpha(ID:3)" {
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
	h := NewAdminHandler(nil, adminmodel.NewAdminRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}), nil, nil, nil)
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
	_ = w.WriteByte('?')
}
func (handlerTestDialector) QuoteTo(w clause.Writer, s string)           { _, _ = w.WriteString("`" + s + "`") }
func (handlerTestDialector) Explain(sql string, _ ...interface{}) string { return sql }

// 验证管理员列表树形组装：按 parent_id 组装
// children，顶层不含父引用孤立节点，叶子保留完整 admin 字段。
func TestAdminTableTreeLeafAssemble(t *testing.T) {
	admins := []*model.Admin{
		{ID: 1, Username: "root", Nickname: "Root", ParentID: nil},
		{ID: 2, Username: "agent", Nickname: "Agent", ParentID: ptr(1)},
		{ID: 3, Username: "leaf", Nickname: "Leaf", ParentID: ptr(2)},
	}
	leaves := make([]*adminTableTreeLeaf, 0, len(admins))
	for _, a := range admins {
		leaves = append(leaves, &adminTableTreeLeaf{Admin: a})
	}
	assembled := tree.AssembleChild(leaves)

	if len(assembled) != 1 {
		t.Fatalf("top level = %d, want 1 (root)", len(assembled))
	}
	root := assembled[0]
	if root.GetId() != 1 || root.Username != "root" {
		t.Fatalf("root leaf = %+v", root)
	}
	children := mustType[[]*adminTableTreeLeaf](t, root.GetChildren())
	if len(children) != 1 || children[0].GetId() != 2 {
		t.Fatalf("root children = %+v", children)
	}
	grand := mustType[[]*adminTableTreeLeaf](t, children[0].GetChildren())
	if len(grand) != 1 || grand[0].GetId() != 3 {
		t.Fatalf("agent children = %+v", grand)
	}
}

// 验证父节点缺失时（搜索/过滤命中的
// 子节点）孤立子节点保留为顶层，不丢失数据。
func TestAdminTableTreeLeafOrphansStayTopLevel(t *testing.T) {
	admins := []*model.Admin{
		{ID: 2, Username: "agent", ParentID: ptr(1)}, // 父 1 不在结果中
		{ID: 3, Username: "leaf", ParentID: ptr(2)},
	}
	leaves := make([]*adminTableTreeLeaf, 0, len(admins))
	for _, a := range admins {
		leaves = append(leaves, &adminTableTreeLeaf{Admin: a})
	}
	assembled := tree.AssembleChild(leaves)
	if len(assembled) != 1 || assembled[0].GetId() != 2 {
		t.Fatalf("orphan must stay top level, got %+v", assembled)
	}
}
