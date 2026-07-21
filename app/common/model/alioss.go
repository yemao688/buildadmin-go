package model

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gin-gonic/gin"
	"go-build-admin/conf"
	"gorm.io/gorm"
)

// AliOSSConfig is deliberately read from ba_config for every operation. This
// mirrors the PHP module and makes an admin edit effective without restart.
type AliOSSConfig struct {
	Mode, Bucket, AccessID, Secret, URL, CDNURL string
}

type AliossStorage struct {
	db    *gorm.DB
	table string
}

// UploadSiteConfig is the public PHP-compatible upload payload. Secret key is
// intentionally used only to sign the policy and is never included in it.
func UploadSiteConfig(ctx *gin.Context, configs interface {
	GetKVByGroup(*gin.Context, string) (map[string]string, error)
}, config *conf.Configuration) (map[string]any, error) {
	values, err := configs.GetKVByGroup(ctx, "upload")
	if err != nil {
		return nil, err
	}
	mode := values["upload_mode"]
	if mode == "" {
		mode = config.Upload.Mode
	}
	allowed := strings.Split(config.Upload.Mimetype, ",")
	result := map[string]any{"maxSize": config.Upload.Maxsize, "saveName": config.Upload.Savename, "allowedSuffixes": allowed, "allowedMimeTypes": allowed, "mode": mode}
	if mode == "alioss" {
		endpoint := values["upload_url"]
		c := AliOSSConfig{Mode: mode, Bucket: values["upload_bucket"], AccessID: values["upload_access_id"], Secret: values["upload_secret_key"], URL: endpoint, CDNURL: values["upload_cdn_url"]}
		policy, signature, expires := PostPolicy(time.Now(), int64(config.Upload.Maxsize), c.Secret)
		result["url"] = uploadURL(c)
		result["params"] = map[string]any{"OSSAccessKeyId": c.AccessID, "policy": policy, "Signature": signature, "Expires": expires}
	}
	return result, nil
}

func uploadURL(c AliOSSConfig) string {
	return "https://" + c.Bucket + "." + strings.TrimPrefix(strings.TrimPrefix(c.URL, "https://"), "http://") + ".aliyuncs.com"
}

func NewAliossStorage(db *gorm.DB, config *conf.Configuration) *AliossStorage {
	return &AliossStorage{db: db, table: config.Database.Prefix + "config"}
}

func (s *AliossStorage) settings() (AliOSSConfig, error) {
	var rows []struct{ Name, Value string }
	err := s.db.Table(s.table).Select("name, value").Where("`group` = ?", "upload").Find(&rows).Error
	if err != nil {
		return AliOSSConfig{}, err
	}
	c := AliOSSConfig{}
	for _, row := range rows {
		switch row.Name {
		case "upload_mode":
			c.Mode = row.Value
		case "upload_bucket":
			c.Bucket = row.Value
		case "upload_access_id":
			c.AccessID = row.Value
		case "upload_secret_key":
			c.Secret = row.Value
		case "upload_url":
			c.URL = row.Value
		case "upload_cdn_url":
			c.CDNURL = row.Value
		}
	}
	return c, nil
}

func endpoint(c AliOSSConfig) string {
	e := c.URL
	if e == "" {
		return ""
	}
	if !strings.Contains(e, ".") {
		e += ".aliyuncs.com"
	}
	if !strings.HasPrefix(e, "http://") && !strings.HasPrefix(e, "https://") {
		e = "https://" + e
	}
	return e
}

func stripURL(name string, c AliOSSConfig) string {
	name = strings.ReplaceAll(name, "\\", "/")
	for _, base := range []string{publicURL(c), uploadURL(c), "//" + strings.TrimPrefix(strings.TrimPrefix(uploadURL(c), "https://"), "http://")} {
		name = strings.TrimPrefix(name, base)
	}
	if parsed, err := url.Parse(name); err == nil && parsed.Host != "" {
		name = parsed.Path
	}
	return strings.TrimLeft(name, "/")
}

func (s *AliossStorage) client(c AliOSSConfig) (*oss.Bucket, error) {
	if c.AccessID == "" || c.Secret == "" || c.Bucket == "" || endpoint(c) == "" {
		return nil, fmt.Errorf("configure Alioss upload parameters first")
	}
	client, err := oss.New(endpoint(c), c.AccessID, c.Secret)
	if err != nil {
		return nil, err
	}
	return client.Bucket(c.Bucket)
}

func normalize(name string, c AliOSSConfig) string {
	return stripURL(name, c)
}

func publicURL(c AliOSSConfig) string {
	base := c.CDNURL
	if base == "" {
		base = "https://" + c.Bucket + "." + strings.TrimPrefix(strings.TrimPrefix(endpoint(c), "https://"), "http://")
	}
	return strings.TrimRight(base, "/") + "/"
}

func (s *AliossStorage) URL(name string) string {
	c, err := s.settings()
	if err != nil {
		return name
	}
	if name == "" {
		return publicURL(c)
	}
	if strings.HasPrefix(name, "http://") || strings.HasPrefix(name, "https://") || strings.HasPrefix(name, "data:image/") {
		return name
	}
	return publicURL(c) + normalize(name, c)
}

func (s *AliossStorage) Save(file io.Reader, saveName string) error {
	c, err := s.settings()
	if err != nil || c.Mode != "alioss" {
		if err != nil {
			return err
		}
		return fmt.Errorf("Alioss upload is disabled")
	}
	b, err := s.client(c)
	if err != nil {
		return err
	}
	return b.PutObject(normalize(saveName, c), file)
}
func (s *AliossStorage) Delete(saveName string) error {
	c, err := s.settings()
	if err != nil {
		return err
	}
	b, err := s.client(c)
	if err != nil {
		return err
	}
	return b.DeleteObject(normalize(saveName, c))
}
func (s *AliossStorage) Exists(saveName string) bool {
	c, err := s.settings()
	if err != nil {
		return false
	}
	b, err := s.client(c)
	if err != nil {
		return false
	}
	_, err = b.GetObjectMeta(normalize(saveName, c))
	return err == nil
}

func PostPolicy(now time.Time, maxSize int64, secret string) (policy, signature string, expires int64) {
	expires = now.Unix() + 3600
	local := now.Add(3600 * time.Second)
	_, offset := local.Zone()
	body := struct {
		Expiration string           `json:"expiration"`
		Conditions [1][]interface{} `json:"conditions"`
	}{fmt.Sprintf("%s.%dZ", local.Format("2006-01-02T15:04:05"), offset), [1][]interface{}{{"content-length-range", 0, maxSize}}}
	raw, _ := json.Marshal(body)
	policy = base64.StdEncoding.EncodeToString(raw)
	h := hmac.New(sha1.New, []byte(secret))
	_, _ = h.Write([]byte(policy))
	signature = base64.StdEncoding.EncodeToString(h.Sum(nil))
	return
}
