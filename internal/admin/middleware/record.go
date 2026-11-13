package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	adminService "buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/util"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type Record struct {
	config    *conf.Configuration
	auditLogM auditLogWriter
}

func NewRecord(config *conf.Configuration, auditLogM *adminService.AuditLogService) *Record {
	return newRecord(config, auditLogM)
}

type auditLogWriter interface {
	Record(context.Context, data_scope.Actor, string, string, string, string, string, map[string]interface{}) error
}

func newRecord(config *conf.Configuration, auditLogM auditLogWriter) *Record {
	return &Record{
		config:    config,
		auditLogM: auditLogM,
	}
}

func (m *Record) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("Timestamp", time.Now().Unix())
		// AutoWriteAdminLog 关闭时完全不读 body（避免无谓的全量缓冲）；
		// 预认证阶段对超大请求体同样跳过参数采集（内存放大 DoS 防线），
		// body 原样保留给下游 handler。
		shouldRecord := m.config.App.AutoWriteAdminLog && isAdminLogMethod(c.Request.Method) && isAdminLogRoute(c.Request.URL.Path)
		params := make(map[string]interface{})
		if shouldRecord {
			if c.Request.Body != nil && c.Request.ContentLength >= 0 && c.Request.ContentLength <= adminLogBodyLimit {
				bodyBytes, _ := io.ReadAll(c.Request.Body)
				c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				mergeRequestParams(c.Request, bodyBytes, params)
			}
		}
		c.Next()
		if !shouldRecord || m.auditLogM == nil {
			return
		}
		info := header.GetAdminAuth(c)
		actor := data_scope.Actor{AdminID: info.Id, Unrestricted: info.IsSuperAdmin}
		if err := m.auditLogM.Record(c.Request.Context(), actor, info.Username, c.Request.URL.Path, c.GetString("log_title"), util.GetClientIP(c), c.Request.UserAgent(), params); err != nil {
			zap.L().Warn("failed to write admin log", zap.Error(err))
		}
	}
}

// adminLogBodyLimit 是 AdminLog 参数采集的请求体上限（8MB）。超过上限时
// 只记录元信息（URL/form/query），避免匿名大 body POST 的内存放大。
const adminLogBodyLimit = 8 << 20

func isAdminLogMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodDelete
}

func isAdminLogRoute(path string) bool {
	return path == "/admin" || strings.HasPrefix(path, "/admin/")
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
