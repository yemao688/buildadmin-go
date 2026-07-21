package model

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go-build-admin/conf"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAdminLogURLFilter(t *testing.T) {
	tests := []struct {
		url  string
		skip bool
	}{
		{"/admin/auth.Admin/index", true},
		{"/admin/auth.Admin/SELECT", true},
		{"/admin/Index/logout", true},
		{"/admin/auth.Admin/add", false},
		{"/admin/auth.Admin/del", false},
		{"/admin/auth.Admin/index/extra", false},
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
	if err := db.AutoMigrate(&AdminLog{}); err != nil {
		t.Fatal(err)
	}
	m := NewAdminLogModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

	for _, url := range []string{"/admin/auth.Admin/index", "/admin/Index/LOGOUT"} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("POST", url, nil)
		m.Add(ctx, map[string]interface{}{})
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.Admin/add", nil)
	m.Add(ctx, map[string]interface{}{})
	var count int64
	if err := db.Model(&AdminLog{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("log count = %d, want 1", count)
	}
}

func TestAdminLogAddUsesLoginUsernameAndUnknownTitle(t *testing.T) {
	db := newAdminLogTestDB(t, "admin-log-details")
	m := NewAdminLogModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	params := map[string]interface{}{"username": "login-user"}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.Admin/login", nil)
	m.Add(ctx, params)

	var row AdminLog
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
	if err := db.Create(&AdminRule{Name: "auth/admin", Title: "管理员"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&AdminRule{Name: "auth/admin/edit", Title: "编辑管理员"}).Error; err != nil {
		t.Fatal(err)
	}
	m := NewAdminLogModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.Admin/edit", nil)
	m.Add(ctx, map[string]interface{}{})

	var row AdminLog
	if err := db.Last(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Title != "管理员-编辑管理员" {
		t.Fatalf("title = %q, want 管理员-编辑管理员", row.Title)
	}
}

func TestAdminLogAddSanitizesNestedParamsAndTruncates(t *testing.T) {
	db := newAdminLogTestDB(t, "admin-log-sanitize")
	m := NewAdminLogModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	params := map[string]interface{}{
		"Password": "top-secret",
		"profile": map[string]interface{}{
			"access_token": "nested-secret",
			"items":        []interface{}{map[string]interface{}{"saltValue": "salt-secret"}},
		},
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.Admin/"+strings.Repeat("x", 1600), nil)
	ctx.Request.Header.Set("User-Agent", strings.Repeat("用户", 200))
	m.Add(ctx, params)

	var row AdminLog
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
	if items[0].(map[string]interface{})["saltValue"] != "***" {
		t.Fatalf("nested slice value was not sanitized: %#v", got)
	}
}

func TestAdminLogAddNilParamsSerializesObject(t *testing.T) {
	db := newAdminLogTestDB(t, "admin-log-nil-params")
	m := NewAdminLogModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.Admin/add", nil)
	m.Add(ctx, nil)
	var row AdminLog
	if err := db.Last(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Data != "{}" {
		t.Fatalf("data = %q, want {}", row.Data)
	}
}

func TestAdminLogAddWithNilModelOrDBDoesNotPanic(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.Admin/add", nil)

	var nilModel *AdminLogModel
	nilModel.Add(ctx, nil)
	(&AdminLogModel{}).Add(ctx, nil)
}

func newAdminLogTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&AdminLog{}, &AdminRule{}); err != nil {
		t.Fatal(err)
	}
	return db
}
