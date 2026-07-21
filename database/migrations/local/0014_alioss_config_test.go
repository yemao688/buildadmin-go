package local

import (
	"encoding/json"
	"testing"
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
