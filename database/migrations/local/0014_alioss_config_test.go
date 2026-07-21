package local

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"go-build-admin/conf"
	"go-build-admin/database/migrations/model"
	"gorm.io/gorm/schema"
)

func TestAliossConfigRowsMatchInstallMetadata(t *testing.T) {
	rows := aliossConfigRows()
	if len(rows) != 6 {
		t.Fatalf("rows=%d", len(rows))
	}
	wants := []struct {
		name, group, title, tip, typ, value, rule string
		id                                        int32
		weigh                                     int32
	}{
		{"upload_mode", "upload", "存储方式", "", "select", "local", "required", 14, 99},
		{"upload_bucket", "upload", "Bucket名称", "请在阿里云对象存储控制台查询", "string", "", "", 15, 98},
		{"upload_access_id", "upload", "AccessKey ID", "请在阿里云个人中心查询", "string", "", "", 16, 97},
		{"upload_secret_key", "upload", "AccessKey Secret", "请在阿里云个人中心查询", "string", "", "", 17, 96},
		{"upload_url", "upload", "存储区域", "请选择存储区域", "select", "", "", 18, 95},
		{"upload_cdn_url", "upload", "CDN地址", "请输入阿里云对象存储的CDN加速域名，以http(s)://开头，比如：https://example.com", "string", "", "", 19, 94},
	}
	for i, want := range wants {
		got := rows[i]
		if got.ID != want.id || got.Name != want.name || got.Group != want.group || got.Title != want.title || got.Tip != want.tip || got.Type != want.typ || got.Value != want.value || got.Rule != want.rule || got.Weigh != want.weigh || got.Extend != "" || got.AllowDel != 0 {
			t.Fatalf("row %d metadata mismatch: %+v", i, got)
		}
	}
	var content map[string]string
	if err := json.Unmarshal([]byte(rows[0].Content), &content); err != nil || content["local"] != "本地磁盘存储" || content["alioss"] != "阿里云对象存储OSS" {
		t.Fatal("upload_mode content mismatch")
	}
	if err := json.Unmarshal([]byte(rows[4].Content), &content); err != nil || len(content) != 38 {
		t.Fatalf("region content count=%d err=%v", len(content), err)
	}
}

func TestAppendUploadConfigGroupIsIdempotent(t *testing.T) {
	original := `[{"key":"basics","value":"Basics"},{"key":"mail","value":"Mail"}]`
	changed, value, err := appendUploadConfigGroup(original)
	if err != nil || !changed {
		t.Fatalf("append changed=%v err=%v", changed, err)
	}
	if value != `[{"key":"basics","value":"Basics"},{"key":"mail","value":"Mail"},{"key":"upload","value":"Upload"}]` {
		t.Fatalf("value=%s", value)
	}
	changed, value, err = appendUploadConfigGroup(value)
	if err != nil || changed || value != `[{"key":"basics","value":"Basics"},{"key":"mail","value":"Mail"},{"key":"upload","value":"Upload"}]` {
		t.Fatalf("second append changed=%v value=%s err=%v", changed, value, err)
	}
}

func TestAppendUploadConfigGroupLeavesInvalidJSONUntouched(t *testing.T) {
	changed, value, err := appendUploadConfigGroup("not-json")
	if err != nil || changed || value != "not-json" {
		t.Fatalf("changed=%v value=%q err=%v", changed, value, err)
	}
}

func TestFreshOverlayAddsUploadConfigGroup(t *testing.T) {
	if os.Getenv("BUILDADMIN_TEST_MYSQL_DSN") == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db := getDB()
	if db == nil {
		t.Fatal("failed to open MySQL test database")
	}
	prefix := fmt.Sprintf("fresh_upload_group_%d_", os.Getpid())
	cfg := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	table := prefix + "config"
	t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS `" + table + "`") })
	if err := db.AutoMigrate(&model.Config{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Config{ID: 1, Name: "config_group", Value: `[{"key":"basics","value":"Basics"}]`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := ensureFreshUploadConfigGroup(db, cfg); err != nil {
		t.Fatal(err)
	}
	if err := ensureFreshUploadConfigGroup(db, cfg); err != nil {
		t.Fatal(err)
	}
	var value string
	if err := db.Table(table).Where("id = 1").Pluck("value", &value).Error; err != nil {
		t.Fatal(err)
	}
	changed, expected, err := appendUploadConfigGroup(`[{"key":"basics","value":"Basics"}]`)
	if err != nil || !changed || value != expected {
		t.Fatalf("value=%s expected=%s changed=%v err=%v", value, expected, changed, err)
	}
}
