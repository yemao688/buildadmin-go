package middleware

import (
	"bytes"
	"encoding/json"
	"go-build-admin/app/admin/model"
	"go-build-admin/conf"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Record struct {
	config    *conf.Configuration
	adminLogM adminLogWriter
}

func NewRecord(config *conf.Configuration, adminLogM *model.AdminLogModel) *Record {
	return newRecord(config, adminLogM)
}

type adminLogWriter interface {
	Add(*gin.Context, map[string]interface{})
}

func newRecord(config *conf.Configuration, adminLogM adminLogWriter) *Record {
	return &Record{
		config:    config,
		adminLogM: adminLogM,
	}
}

func (m *Record) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("Timestamp", time.Now().Unix())
		params := make(map[string]interface{})
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodDelete {
			var bodyBytes []byte
			if c.Request.Body != nil {
				bodyBytes, _ = io.ReadAll(c.Request.Body)
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			mergeRequestParams(c.Request, bodyBytes, params)
		}
		c.Next()
		if (c.Request.Method == http.MethodPost || c.Request.Method == http.MethodDelete) && m.config.App.AutoWriteAdminLog {
			m.adminLogM.Add(c, params)
		}
	}
}

// mergeRequestParams keeps the request body available to downstream handlers while
// collecting the same request inputs that are useful for an admin operation log.
// Later sources intentionally override earlier ones: form values override query
// values, and JSON object fields override both.
func mergeRequestParams(req *http.Request, bodyBytes []byte, params map[string]interface{}) {
	mergeValues(params, req.URL.Query())

	mediaType, _, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if mediaType == "application/x-www-form-urlencoded" || mediaType == "multipart/form-data" {
		formReq := req.Clone(req.Context())
		formReq.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		formReq.ContentLength = int64(len(bodyBytes))
		if err := formReq.ParseMultipartForm(32 << 20); err == nil {
			mergeValues(params, formReq.PostForm)
		}
	}

	if len(bodyBytes) == 0 {
		return
	}
	var jsonParams map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &jsonParams); err == nil && jsonParams != nil {
		for key, value := range jsonParams {
			params[key] = value
		}
	}
}

func mergeValues(params map[string]interface{}, values map[string][]string) {
	for key, items := range values {
		if len(items) > 1 || strings.HasSuffix(key, "[]") {
			params[key] = append([]string(nil), items...)
		} else if len(items) == 1 {
			params[key] = items[0]
		}
	}
}
