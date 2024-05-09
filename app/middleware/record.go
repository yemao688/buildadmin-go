package middleware

import (
	"bytes"
	"encoding/json"
	"go-build-admin/app/admin/model"
	"go-build-admin/conf"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Record struct {
	config    *conf.Configuration
	adminLogM *model.AdminLogModel
}

func NewRecord(config *conf.Configuration, adminLogM *model.AdminLogModel) *Record {
	return &Record{
		config:    config,
		adminLogM: adminLogM,
	}
}

func (m *Record) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("Timestamp", time.Now().Unix())
		c.Next()
		if (c.Request.Method == http.MethodPost || c.Request.Method == http.MethodDelete) && m.config.App.AutoWriteAdminLog {
			bodyBytes, _ := io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			var params map[string]interface{}
			json.Unmarshal(bodyBytes, &params)
			m.adminLogM.Add(c, params)
		}
	}
}
