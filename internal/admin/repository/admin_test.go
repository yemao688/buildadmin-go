package repository

import (
	"context"
	"errors"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// testDialector is a minimal in-memory dialector for unit tests.
// It is duplicated from internal/pkg/data_scope so that model tests do not
// need a running MySQL instance.
type testDialector struct{}

func (testDialector) Name() string                                   { return "test" }
func (testDialector) Initialize(*gorm.DB) error                      { return nil }
func (testDialector) Migrator(*gorm.DB) gorm.Migrator                { return nil }
func (testDialector) DataTypeOf(*schema.Field) string                { return "" }
func (testDialector) DefaultValueOf(*schema.Field) clause.Expression { return nil }
func (testDialector) BindVarTo(w clause.Writer, stmt *gorm.Statement, v interface{}) {
	_ = stmt
	_ = v
	_ = w.WriteByte('?')
}
func (testDialector) QuoteTo(w clause.Writer, s string)           { _, _ = w.WriteString("`" + s + "`") }
func (testDialector) Explain(sql string, _ ...interface{}) string { return sql }

func openAdminTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(testDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return db
}

func adminTestContext(t *testing.T, unrestricted bool) *gin.Context {
	t.Helper()
	return adminTestContextID(t, 5, unrestricted)
}

func adminTestContextID(t *testing.T, id int32, unrestricted bool) *gin.Context {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.model.Admin/edit", nil)
	var actor data_scope.Actor
	var err error
	if unrestricted {
		actor, err = data_scope.NewUnrestrictedActor(id)
	} else {
		actor, err = data_scope.NewActor(id)
	}
	if err != nil {
		t.Fatalf("actor: %v", err)
	}
	ctx.Set(data_scope.ActorContextKey, actor)
	return ctx
}

func ptr(v int32) *int32 { return &v }

func TestAdminModelScopedFailClosed(t *testing.T) {
	t.Parallel()
	db := openAdminTestDB(t)
	m := NewAdminRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

	ctx, _ := gin.CreateTestContext(nil)
	scoped := m.scoped(ctx)(db.Model(&model.Admin{}))
	if !errors.Is(scoped.Error, data_scope.ErrScopedAccessDenied) {
		t.Fatalf("missing actor should fail closed, got %v", scoped.Error)
	}
}

func TestAdminModelScopedUnrestrictedBypass(t *testing.T) {
	t.Parallel()
	db := openAdminTestDB(t)
	m := NewAdminRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

	ctx := adminTestContext(t, true)
	scoped := m.scoped(ctx)(db.Model(&model.Admin{}))
	if scoped.Error != nil {
		t.Fatalf("unrestricted actor should receive clean DB, got %v", scoped.Error)
	}
}

func TestAdminModelActorRequiresRealRequest(t *testing.T) {
	t.Parallel()
	db := openAdminTestDB(t)
	m := NewAdminRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	if _, err := m.actor(nil); !errors.Is(err, data_scope.ErrScopedAccessDenied) {
		t.Fatalf("nil context should fail closed, got %v", err)
	}
	ctx, _ := gin.CreateTestContext(nil)
	if _, err := m.actor(ctx); !errors.Is(err, data_scope.ErrScopedAccessDenied) {
		t.Fatalf("nil request should fail closed, got %v", err)
	}
}

func TestLoadParentSummariesSelectsUsername(t *testing.T) {
	t.Parallel()
	db := openAdminTestDB(t)
	m := NewAdminRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	ctx := adminTestContext(t, true)
	child := &model.Admin{ID: 10, ParentID: ptr(5)}

	var captured []string
	callbackName := "capture_admin_parent_summary_select"
	if err := db.Callback().Query().Before("gorm:query").Register(callbackName, func(tx *gorm.DB) {
		captured = append([]string(nil), tx.Statement.Selects...)
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(callbackName)
	})

	if err := m.loadParentSummaries(ctx, db, m.scoped(ctx), []*model.Admin{child}); err != nil {
		t.Fatalf("loadParentSummaries: %v", err)
	}
	if !slices.Contains(captured, "id") || !slices.Contains(captured, "nickname") || !slices.Contains(captured, "username") {
		t.Fatalf("captured selects = %v, want id/nickname/username", captured)
	}
}

// adminGroupSQLCounter 统计命中的 admin_group_access SELECT 次数，
// 用于断言列表组信息装载是单次批量查询而非逐行 N+1。
type adminGroupSQLCounter struct {
	logger.Interface
	groupAccessSelects int
}

func (l *adminGroupSQLCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	if strings.Contains(sql, "admin_group_access") && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "SELECT") {
		l.groupAccessSelects++
	}
}

func TestLoadGroupSummariesBatchesOneQuery(t *testing.T) {
	t.Parallel()
	counter := &adminGroupSQLCounter{Interface: logger.Default.LogMode(logger.Silent)}
	db, err := gorm.Open(sqlite.Open("file:admin-group-summaries-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
		Logger:         counter,
	})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteAdminRuleTables(db, "ba_admin_rule", "ba_admin_group", "ba_admin_group_access"))

	groups := []model.AdminGroup{
		{ID: 1, Name: "super"},
		{ID: 2, Name: "ops"},
		{ID: 3, Name: "audit"},
	}
	require.NoError(t, db.Create(&groups).Error)
	// 故意按非 group_id 序插入（2,1），验证批处理保留逐行查询的返回顺序。
	access := []model.AdminGroupAccess{
		{UID: 1, GroupID: 2},
		{UID: 1, GroupID: 1},
		{UID: 3, GroupID: 3},
	}
	require.NoError(t, db.Create(&access).Error)

	m := NewAdminRepository(db, &conf.Configuration{
		Database: conf.Database{Prefix: "ba_"},
		Token:    conf.Token{Key: "test-key"},
		App:      conf.App{DefaultAvatar: ""},
	})
	ctx := context.Background()
	admins := []*model.Admin{{ID: 1}, {ID: 2}, {ID: 3}}
	require.NoError(t, m.loadGroupSummaries(ctx, db, admins))

	// 3 行管理员只允许 1 次 admin_group_access 查询（原先为 3 次）。
	require.Equal(t, 1, counter.groupAccessSelects, "expected one batched group query")

	require.Equal(t, []int32{2, 1}, admins[0].GroupArr)
	require.Equal(t, []string{"ops", "super"}, admins[0].GroupNameArr)
	// 无分组的行保持空数组（与逐行 DealData 的兜底一致）。
	require.Empty(t, admins[1].GroupArr)
	require.Empty(t, admins[1].GroupNameArr)
	require.Equal(t, []int32{3}, admins[2].GroupArr)
	require.Equal(t, []string{"audit"}, admins[2].GroupNameArr)

	// 与单行路径 DealData 输出逐字段一致（同一访问路径下逐行顺序相同）。
	ref := &model.Admin{ID: 1}
	require.NoError(t, m.DealData(ctx, ref))
	require.Equal(t, ref.GroupArr, admins[0].GroupArr)
	require.Equal(t, ref.GroupNameArr, admins[0].GroupNameArr)

	// 边界：空列表 / 零 id / nil 行跳过查询，避免 IN () 语法错误。
	before := counter.groupAccessSelects
	require.NoError(t, m.loadGroupSummaries(ctx, db, nil))
	require.NoError(t, m.loadGroupSummaries(ctx, db, []*model.Admin{nil, {ID: 0}}))
	require.Equal(t, before, counter.groupAccessSelects)
}
