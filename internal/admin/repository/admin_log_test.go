package repository

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAdminLogURLFilter(t *testing.T) {
	tests := []struct {
		url  string
		skip bool
	}{
		{"/admin/auth.model.Admin/index", true},
		{"/admin/auth.model.Admin/SELECT", true},
		{"/admin/Index/logout", true},
		{"/admin/auth.model.Admin/add", false},
		{"/admin/auth.model.Admin/del", false},
		{"/admin/auth.model.Admin/index/extra", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := skipAdminLogURL(tt.url); got != tt.skip {
				t.Fatalf("skipAdminLogURL(%q) = %v, want %v", tt.url, got, tt.skip)
			}
		})
	}
}

func TestAdminLogAddFiltersURLSuffixes(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-log-filter?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture table with
	// sqlite-native DDL matching the runtime column shape.
	if err := testutil.CreateSQLiteAdminLogTable(db, "ba_admin_log"); err != nil {
		t.Fatal(err)
	}
	m := NewAdminLogRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, nil)

	for _, url := range []string{"/admin/auth.model.Admin/index", "/admin/Index/LOGOUT"} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("POST", url, nil)
		m.Add(ctx, map[string]interface{}{})
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.model.Admin/add", nil)
	m.Add(ctx, map[string]interface{}{})
	var count int64
	if err := db.Model(&model.AdminLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("log count = %d, want 1", count)
	}
}

func TestAdminLogAddUsesLoginUsernameAndUnknownTitle(t *testing.T) {
	db := newAdminLogTestDB(t, "admin-log-details")
	m := NewAdminLogRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, nil)
	params := map[string]interface{}{"username": "login-user"}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.model.Admin/login", nil)
	m.Add(ctx, params)

	var row model.AdminLog
	if err := db.Last(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Username != "login-user" {
		t.Fatalf("username = %q, want login-user", row.Username)
	}
	if row.Title != "Unknown(login)" {
		t.Fatalf("title = %q, want Unknown(login)", row.Title)
	}
}

func TestAdminLogAddUsesRuleTitles(t *testing.T) {
	db := newAdminLogTestDB(t, "admin-log-rule-titles")
	if err := db.Create(&model.AdminRule{Name: "auth/admin", Title: "管理员"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.AdminRule{Name: "auth/admin/edit", Title: "编辑管理员"}).Error; err != nil {
		t.Fatal(err)
	}
	m := NewAdminLogRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.Admin/edit", nil)
	m.Add(ctx, map[string]interface{}{})

	var row model.AdminLog
	if err := db.Last(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Title != "管理员-编辑管理员" {
		t.Fatalf("title = %q, want 管理员-编辑管理员", row.Title)
	}
}

func TestAdminLogAddUsesCachedRuleTitles(t *testing.T) {
	authM, db, initialRule := newAdminAuthCacheModel(t)
	parent := model.AdminRule{Pid: 0, Type: "menu", Title: "管理员", Name: "auth/admin", Status: "1", Weigh: 2}
	action := model.AdminRule{Pid: 0, Type: "button", Title: "编辑管理员", Name: "auth/admin/edit", Status: "1", Weigh: 3}
	require.NoError(t, db.Create(&parent).Error)
	require.NoError(t, db.Create(&action).Error)
	require.NoError(t, db.Model(&model.AdminGroup{}).Where("id=?", 1).Update("rules",
		strconv.Itoa(int(initialRule.ID))+","+strconv.Itoa(int(parent.ID))+","+strconv.Itoa(int(action.ID))).Error)
	_, err := authM.GetRuleList(nil, 1)
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture table with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteAdminLogTable(db, "admin_log"))

	// Remove the source rows after warming the permission cache. A cache miss
	// would make Add fall back to the database and lose both titles.
	require.NoError(t, db.Where("id IN ?", []int32{parent.ID, action.ID}).Delete(&model.AdminRule{}).Error)
	m := NewAdminLogRepository(db, &conf.Configuration{}, authM)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.Admin/edit", nil)
	header.SetAdminAuth(ctx, header.AdminAuth{Id: 1, Username: "cached-admin"})
	m.Add(ctx, nil)

	var row model.AdminLog
	require.NoError(t, db.Last(&row).Error)
	require.Equal(t, "管理员-编辑管理员", row.Title)
}

func TestAdminLogAddSanitizesNestedParamsAndTruncates(t *testing.T) {
	db := newAdminLogTestDB(t, "admin-log-sanitize")
	m := NewAdminLogRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, nil)
	params := map[string]interface{}{
		"Password": "top-secret",
		"profile": map[string]interface{}{
			"access_token": "nested-secret",
			"items":        []interface{}{map[string]interface{}{"saltValue": "salt-secret"}},
		},
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.model.Admin/"+strings.Repeat("x", 1600), nil)
	ctx.Request.Header.Set("User-Agent", strings.Repeat("用户", 200))
	m.Add(ctx, params)

	var row model.AdminLog
	if err := db.Last(&row).Error; err != nil {
		t.Fatal(err)
	}
	if len(row.URL) > 1500 || !json.Valid([]byte(row.Data)) {
		t.Fatalf("invalid URL/data: url bytes=%d data=%q", len(row.URL), row.Data)
	}
	if len(row.Useragent) > 255 {
		t.Fatalf("useragent bytes=%d, want <=255", len(row.Useragent))
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(row.Data), &got); err != nil {
		t.Fatal(err)
	}
	if got["Password"] != "***" || got["profile"].(map[string]interface{})["access_token"] != "***" {
		t.Fatalf("sensitive values were not sanitized: %#v", got)
	}
	items := got["profile"].(map[string]interface{})["items"].([]interface{})
	if items[0].(map[string]interface{})["saltValue"] != "salt-secret" {
		t.Fatalf("removed salt field was sanitized: %#v", got)
	}
}

func TestAdminLogAddNilParamsSerializesObject(t *testing.T) {
	db := newAdminLogTestDB(t, "admin-log-nil-params")
	m := NewAdminLogRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, nil)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.model.Admin/add", nil)
	m.Add(ctx, nil)
	var row model.AdminLog
	if err := db.Last(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Data != "{}" {
		t.Fatalf("data = %q, want {}", row.Data)
	}
}

func TestAdminLogAddWithNilModelOrDBDoesNotPanic(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.model.Admin/add", nil)

	var nilModel *AdminLogRepository
	nilModel.Add(ctx, nil)
	(&AdminLogRepository{}).Add(ctx, nil)
}

func newAdminLogTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The entities carry MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	if err := testutil.CreateSQLiteAdminLogTable(db, "ba_admin_log"); err != nil {
		t.Fatal(err)
	}
	if err := testutil.CreateSQLiteAdminRuleTables(db, "ba_admin_rule", "ba_admin_group", "ba_admin_group_access"); err != nil {
		t.Fatal(err)
	}
	return db
}
