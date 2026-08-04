package upload

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"buildadmin-go/internal/conf"
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
