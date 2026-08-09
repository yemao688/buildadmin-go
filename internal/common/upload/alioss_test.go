package upload

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/testutil"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestPostPolicyFixedTime(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	policy, signature, expires := PostPolicy(now, 10485760, "secret")
	if expires != 1700003600 {
		t.Fatalf("expires=%d", expires)
	}
	raw, err := base64.StdEncoding.DecodeString(policy)
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Expiration string          `json:"expiration"`
		Conditions [][]interface{} `json:"conditions"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.Expiration != "2023-11-14T23:13:20.0Z" {
		t.Fatalf("expiration=%q", body.Expiration)
	}
	h := hmac.New(sha1.New, []byte("secret"))
	h.Write([]byte(policy))
	if signature != base64.StdEncoding.EncodeToString(h.Sum(nil)) {
		t.Fatal("signature mismatch")
	}
}

func TestUploadSiteConfigEmptySecretAndFields(t *testing.T) {
	// Use the concrete adapter below because UploadSiteConfig intentionally
	// accepts ConfigModel's method shape.
	values := configValues{"upload_mode": "alioss", "upload_bucket": "demo", "upload_url": "oss-cn-hangzhou"}
	result, err := UploadSiteConfig(nil, values, &conf.Configuration{Upload: conf.Upload{Maxsize: 10, Savename: "/x/{fileName}", Mimetype: "jpg,png"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result["saveName"].(string); !ok {
		t.Fatalf("saveName type=%T", result["saveName"])
	}
	if result["url"] != "https://demo.oss-cn-hangzhou.aliyuncs.com" {
		t.Fatalf("url=%v", result["url"])
	}
	// alioss 直传档：未配置 upload_cdn_url 时 cdn 回退 bucketUrl（与 url 相同）
	if result["cdn"] != "https://demo.oss-cn-hangzhou.aliyuncs.com" {
		t.Fatalf("cdn=%v", result["cdn"])
	}
	for _, key := range []string{"allowedSuffixes", "allowedMimeTypes", "maxSize", "mode", "params", "cdn"} {
		if _, ok := result[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
}

func TestUploadSiteConfigCDNPriority(t *testing.T) {
	// upload_cdn_url 配置时优先于 bucketUrl
	values := configValues{"upload_mode": "alioss", "upload_bucket": "demo", "upload_url": "oss-cn-hangzhou", "upload_cdn_url": "https://cdn.example.com"}
	result, err := UploadSiteConfig(nil, values, &conf.Configuration{Upload: conf.Upload{Maxsize: 10, Savename: "/x/{fileName}", Mimetype: "jpg,png"}})
	if err != nil {
		t.Fatal(err)
	}
	if result["cdn"] != "https://cdn.example.com" {
		t.Fatalf("cdn=%v, want upload_cdn_url", result["cdn"])
	}

	// 非 alioss 模式：不设置 cdn 键（与 PHP request->upload 仅在 alioss 时存在一致）
	values = configValues{"upload_mode": "local"}
	result, err = UploadSiteConfig(nil, values, &conf.Configuration{Upload: conf.Upload{Maxsize: 10, Savename: "/x/{fileName}", Mimetype: "jpg,png"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := result["cdn"]; ok {
		t.Fatal("cdn must be absent when upload mode is not alioss")
	}
}

type configValues map[string]string

func (v configValues) GetKVByGroup(_ *gin.Context, _ string) (map[string]string, error) {
	return v, nil
}

func TestCleanUploadNameAndCallbackURL(t *testing.T) {
	name := cleanUploadName("<b>a&amp;</b>")
	if name != "a&amp;amp;" {
		t.Fatalf("name=%q", name)
	}
	path := normalizeCallbackURL("\\foo/bar")
	if path != "/foo/bar" {
		t.Fatalf("path=%q", path)
	}
	if got := publicURL(AliOSSConfig{Bucket: "demo", URL: "oss-cn-hangzhou"}); got != "https://demo.oss-cn-hangzhou.aliyuncs.com/" {
		t.Fatalf("empty-name URL base=%q", got)
	}
}

func TestURLWithConfig(t *testing.T) {
	s := &AliossStorage{}
	c := AliOSSConfig{Mode: "alioss", Bucket: "demo", URL: "oss-cn-hangzhou", CDNURL: "https://cdn.example.com"}

	// 相对路径：cdn 基址 + 规范化路径。
	if got := s.URLWithConfig("/storage/a.png", c); got != "https://cdn.example.com/storage/a.png" {
		t.Fatalf("relative path URL=%q", got)
	}
	// 反斜杠路径规范化（stripURL 的 \\ → /）。
	if got := s.URLWithConfig("\\storage\\a.png", c); got != "https://cdn.example.com/storage/a.png" {
		t.Fatalf("backslash path URL=%q", got)
	}
	// 空 name：返回公开基址。
	if got := s.URLWithConfig("", c); got != "https://cdn.example.com/" {
		t.Fatalf("empty name URL=%q", got)
	}
	// 绝对 URL / data:image 直通。
	for _, name := range []string{"https://x.com/a.png", "http://x.com/a.png", "data:image/png;base64,AAA"} {
		if got := s.URLWithConfig(name, c); got != name {
			t.Fatalf("passthrough URL(%q)=%q", name, got)
		}
	}
	// 未配 CDN：回退 bucket URL。
	noCDN := AliOSSConfig{Mode: "alioss", Bucket: "demo", URL: "oss-cn-hangzhou"}
	if got := s.URLWithConfig("/a.png", noCDN); got != "https://demo.oss-cn-hangzhou.aliyuncs.com/a.png" {
		t.Fatalf("no-cdn URL=%q", got)
	}
	// 未配 CDN 的空 name：bucket 基址。
	if got := s.URLWithConfig("", noCDN); got != "https://demo.oss-cn-hangzhou.aliyuncs.com/" {
		t.Fatalf("no-cdn empty-name URL=%q", got)
	}
}

// TestURLWithConfigMatchesURL 断言批处理方法（传入 settings）与单行路径
// URL()（每次自读 settings）输出逐值一致——批处理前提是 settings 查询结果
// 与行数据无关（固定 group='upload' 过滤），此处同时验证两次读取结果相同。
func TestURLWithConfigMatchesURL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:upload-alioss-parity-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteConfigTable(db, "ba_config"))
	require.NoError(t, db.Exec(`INSERT INTO ba_config (name, "group", value) VALUES
		('upload_mode','upload','alioss'),
		('upload_bucket','upload','demo'),
		('upload_url','upload','oss-cn-hangzhou'),
		('upload_cdn_url','upload','https://cdn.example.com')`).Error)

	s := NewAliossStorage(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	c, err := s.Settings()
	require.NoError(t, err)
	require.Equal(t, "alioss", c.Mode)

	for _, name := range []string{"/storage/a.png", "\\storage\\b.png", "", "https://x.com/a.png", "data:image/png;base64,AAA"} {
		require.Equal(t, s.URL(name), s.URLWithConfig(name, c), "name=%q", name)
	}

	// settings 固定查询：两次读取结果逐字段相同（批处理前提）。
	c2, err := s.Settings()
	require.NoError(t, err)
	require.Equal(t, c, c2)
}
