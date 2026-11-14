package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	adminrepo "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func newAuditLogServiceFixture(t *testing.T) (*AuditLogService, *gorm.DB, *adminrepo.AuthRepository) {
	t.Helper()
	dsn := "file:audit-log-service-" + strings.ReplaceAll(t.Name(), "/", "-") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteAdminLogTable(db, "admin_log"))
	require.NoError(t, testutil.CreateSQLiteAdminRuleTables(db, "admin_rule", "admin_group", "admin_group_access"))
	// 操作日志只读 admin.username；共享实体携带 MySQL 方言 tag，sqlite 下用
	// 最小列集建表。
	require.NoError(t, db.Exec("CREATE TABLE IF NOT EXISTS admin (id INTEGER PRIMARY KEY, username TEXT NOT NULL DEFAULT '', nickname TEXT NOT NULL DEFAULT '')").Error)

	config := &conf.Configuration{}
	authM := adminrepo.NewAuthRepository(db, nil, config)
	svc := NewAuditLogService(
		adminrepo.NewAdminLogRepository(db, config),
		adminrepo.NewAdminRuleRepository(db, config),
		authM,
	)
	return svc, db, authM
}

func lastAuditLog(t *testing.T, db *gorm.DB) model.AdminLog {
	t.Helper()
	var row model.AdminLog
	require.NoError(t, db.Last(&row).Error)
	return row
}

func TestAuditLogServiceSkipsListSelectAndLogoutURLs(t *testing.T) {
	t.Parallel()
	svc, db, _ := newAuditLogServiceFixture(t)
	ctx := context.Background()
	countLogs := func() int64 {
		var count int64
		require.NoError(t, db.Model(&model.AdminLog{}).Count(&count).Error)
		return count
	}

	for _, url := range []string{
		"/admin/auth.model.Admin/index",
		"/admin/Index/LOGOUT",
		"/admin/auth.model.Admin/select",
	} {
		require.NoError(t, svc.Record(ctx, data_scope.Actor{AdminID: 1}, "root", url, "", "203.0.113.9", "ua", map[string]interface{}{}))
		require.Zero(t, countLogs(), "url %s must be skipped", url)
	}

	// 后缀不是 index/select/logout 的路径必须落库，确认跳过规则没有放宽。
	require.NoError(t, svc.Record(ctx, data_scope.Actor{AdminID: 1}, "root", "/admin/auth.model.Admin/index/extra", "", "203.0.113.9", "ua", map[string]interface{}{}))
	require.EqualValues(t, 1, countLogs())
	require.NoError(t, svc.Record(ctx, data_scope.Actor{AdminID: 1}, "root", "/admin/auth.model.Admin/add", "", "203.0.113.9", "ua", map[string]interface{}{}))
	require.EqualValues(t, 2, countLogs())
}

func TestAuditLogServiceResolvesUsernameFallbacks(t *testing.T) {
	t.Parallel()
	svc, db, _ := newAuditLogServiceFixture(t)
	require.NoError(t, db.Exec("INSERT INTO admin (id, username) VALUES (?, ?)", 7, "db-admin").Error)

	tests := []struct {
		name     string
		actor    data_scope.Actor
		username string
		params   map[string]interface{}
		want     string
	}{
		{name: "login username", actor: data_scope.Actor{AdminID: 7}, username: "login-user", want: "login-user"},
		{name: "admin table", actor: data_scope.Actor{AdminID: 7}, want: "db-admin"},
		{name: "request param", actor: data_scope.Actor{}, params: map[string]interface{}{"username": "param-user"}, want: "param-user"},
		{name: "unknown", actor: data_scope.Actor{}, want: "Unknown"},
		{name: "missing admin row", actor: data_scope.Actor{AdminID: 99}, want: "Unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, svc.Record(context.Background(), test.actor, test.username, "/admin/auth.Admin/add", "标题", "203.0.113.9", "ua", test.params))
			require.Equal(t, test.want, lastAuditLog(t, db).Username)
		})
	}
}

func TestAuditLogServiceResolvesRuleTitles(t *testing.T) {
	t.Parallel()
	svc, db, _ := newAuditLogServiceFixture(t)
	require.NoError(t, db.Create(&model.AdminRule{Name: "auth/admin", Title: "管理员"}).Error)
	require.NoError(t, db.Create(&model.AdminRule{Name: "auth/admin/edit", Title: "编辑管理员"}).Error)
	ctx := context.Background()

	require.NoError(t, svc.Record(ctx, data_scope.Actor{AdminID: 1}, "root", "/admin/auth.Admin/edit", "", "203.0.113.9", "ua", nil))
	require.Equal(t, "管理员-编辑管理员", lastAuditLog(t, db).Title)

	require.NoError(t, svc.Record(ctx, data_scope.Actor{AdminID: 1}, "root", "/admin/auth.model.Admin/login", "", "203.0.113.9", "ua", nil))
	require.Equal(t, "Unknown(login)", lastAuditLog(t, db).Title)

	require.NoError(t, svc.Record(ctx, data_scope.Actor{AdminID: 1}, "root", "/admin/auth.Admin/add", "", "203.0.113.9", "ua", nil))
	require.Equal(t, "Unknown(add)", lastAuditLog(t, db).Title)

	require.NoError(t, svc.Record(ctx, data_scope.Actor{AdminID: 1}, "root", "/admin/auth.Admin/add", "显式标题", "203.0.113.9", "ua", nil))
	require.Equal(t, "显式标题", lastAuditLog(t, db).Title)
}

func TestAuditLogServiceUsesCachedRuleTitles(t *testing.T) {
	t.Parallel()
	svc, db, authM := newAuditLogServiceFixture(t)
	initial := model.AdminRule{Pid: 0, Type: "menu", Title: "Initial", Name: "auth/initial", Status: "1", Weigh: 1}
	parent := model.AdminRule{Pid: 0, Type: "menu", Title: "管理员", Name: "auth/admin", Status: "1", Weigh: 2}
	action := model.AdminRule{Pid: 0, Type: "button", Title: "编辑管理员", Name: "auth/admin/edit", Status: "1", Weigh: 3}
	for _, rule := range []*model.AdminRule{&initial, &parent, &action} {
		require.NoError(t, db.Create(rule).Error)
	}
	require.NoError(t, db.Create(&model.AdminGroup{ID: 1, Name: "operators", Rules: strings.Join([]string{
		strconv.Itoa(int(initial.ID)), strconv.Itoa(int(parent.ID)), strconv.Itoa(int(action.ID)),
	}, ","), Status: "1"}).Error)
	require.NoError(t, db.Create(&model.AdminGroupAccess{UID: 1, GroupID: 1}).Error)
	_, err := authM.GetRuleList(nil, 1)
	require.NoError(t, err)

	// 缓存已预热，删除源行后仍应取到缓存标题（证明优先走 CachedRuleTitle）。
	require.NoError(t, db.Where("id IN ?", []int32{parent.ID, action.ID}).Delete(&model.AdminRule{}).Error)

	require.NoError(t, svc.Record(context.Background(), data_scope.Actor{AdminID: 1}, "", "/admin/auth.Admin/edit", "", "203.0.113.9", "ua", nil))
	require.Equal(t, "管理员-编辑管理员", lastAuditLog(t, db).Title)
}

func TestAuditLogServiceSanitizesAndTruncates(t *testing.T) {
	t.Parallel()
	svc, db, _ := newAuditLogServiceFixture(t)
	params := map[string]interface{}{
		"Password": "top-secret",
		"profile": map[string]interface{}{
			"access_token": "nested-secret",
			"items":        []interface{}{map[string]interface{}{"saltValue": "salt-secret"}},
		},
	}
	url := "/admin/auth.model.Admin/" + strings.Repeat("x", 1600)
	userAgent := strings.Repeat("用户", 200)
	require.NoError(t, svc.Record(context.Background(), data_scope.Actor{AdminID: 3}, "", url, "", "198.51.100.7", userAgent, params))

	row := lastAuditLog(t, db)
	require.LessOrEqual(t, len(row.URL), 1500)
	require.LessOrEqual(t, len(row.Useragent), 255)
	require.Equal(t, "198.51.100.7", row.IP)
	require.True(t, json.Valid([]byte(row.Data)), "data must stay valid JSON: %q", row.Data)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(row.Data), &got))
	profile, ok := got["profile"].(map[string]interface{})
	require.True(t, ok, "profile = %#v", got["profile"])
	require.Equal(t, "***", got["Password"])
	require.Equal(t, "***", profile["access_token"])
	items, ok := profile["items"].([]interface{})
	require.True(t, ok, "items = %#v", profile["items"])
	firstItem, ok := items[0].(map[string]interface{})
	require.True(t, ok, "items[0] = %#v", items[0])
	// saltValue 不在 password|token 规则内，必须保持原值（只脱敏指定字段）。
	require.Equal(t, "salt-secret", firstItem["saltValue"])
}

func TestAuditLogServiceSerializesNilParamsAsObject(t *testing.T) {
	t.Parallel()
	svc, db, _ := newAuditLogServiceFixture(t)
	require.NoError(t, svc.Record(context.Background(), data_scope.Actor{AdminID: 1}, "root", "/admin/auth.Admin/add", "标题", "", "", nil))
	require.Equal(t, "{}", lastAuditLog(t, db).Data)
}

func TestAuditLogServiceNilDependenciesDoNotPanic(t *testing.T) {
	t.Parallel()
	var nilService *AuditLogService
	require.NoError(t, nilService.Record(context.Background(), data_scope.Actor{}, "", "/admin/auth.Admin/add", "", "", "", nil))

	empty := &AuditLogService{}
	require.NoError(t, empty.Record(context.Background(), data_scope.Actor{}, "", "/admin/auth.Admin/add", "", "", "", nil))
}
