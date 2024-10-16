package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/header"
	"go-build-admin/conf"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/unknwon/com"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Security struct {
	config *conf.Configuration
	log    *zap.Logger
	sqlDB  *gorm.DB
}

func NewSecurity(
	config *conf.Configuration,
	log *zap.Logger,
	sqlDB *gorm.DB,
) *Security {
	return &Security{
		config: config,
		log:    log,
		sqlDB:  sqlDB,
	}
}

func (m *Security) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		getPath := func(c *gin.Context) string {
			path := ""
			parts := strings.Split(c.FullPath(), "/")
			if len(parts) >= 3 {
				path = parts[1] + "/" + parts[2]
			}
			return path
		}

		//记录删除数据操作
		if c.Request.Method == http.MethodDelete {
			//是否配置回收规则
			recycle := model.SecurityDataRecycle{}
			err := m.sqlDB.Model(&model.SecurityDataRecycle{}).Where(" status=? and controller_as=?", "1", getPath(c)).First(&recycle).Error
			if err != nil {
				return
			}

			var params = struct {
				Ids []int32 `form:"ids[]" binding:"required"`
			}{}
			c.ShouldBindQuery(&params)

			rows := []map[string]any{}
			err = m.sqlDB.Table(m.config.Database.Prefix+recycle.DataTable).Where(recycle.PrimaryKey+" in ? ", params.Ids).Scan(&rows).Error
			if err != nil {
				m.log.Warn("[ DataSecurity ] Failed to recycle data:" + err.Error())
				return
			}

			//创建删除记录
			admin := header.GetAdminAuth(c)
			recycleLogs := []model.SecurityDataRecycleLog{}
			for _, v := range rows {
				data, _ := json.Marshal(v)
				recycleLogs = append(recycleLogs, model.SecurityDataRecycleLog{
					AdminID:    admin.Id,
					RecycleID:  recycle.ID,
					Data:       string(data),
					DataTable:  recycle.DataTable,
					PrimaryKey: recycle.PrimaryKey,
					IP:         c.ClientIP(),
					Useragent:  c.Request.Header.Get("User-Agent"),
				})
			}
			if len(recycleLogs) == 0 {
				return
			}

			err = m.sqlDB.Model(&model.SecurityDataRecycleLog{}).Create(&recycleLogs).Error
			if err != nil {
				m.log.Warn("[ DataSecurity ] Failed to recycle data:" + err.Error())
				return
			}
			return
		}

		// 记录修改数据操作
		if c.Request.Method == http.MethodPost {
			//是否配置敏感数据规则
			sensitive := model.SecuritySensitiveData{}
			err := m.sqlDB.Model(&model.SecuritySensitiveData{}).Where(" status=? and controller_as=?", "1", getPath(c)).First(&sensitive).Error
			if err != nil {
				return
			}

			//读取请求参数
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read request body"})
				c.Abort()
				return
			}

			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))
			var params map[string]interface{}
			if err := json.Unmarshal(body, &params); err != nil {
				return
			}
			if _, ok := params["id"]; !ok {
				return
			}

			//查询需要修改的记录
			row := map[string]any{}
			err = m.sqlDB.Table(m.config.Database.Prefix+sensitive.DataTable).Where(sensitive.PrimaryKey+"=?", params["id"]).Scan(&row).Error
			if err != nil {
				m.log.Warn("[ DataSecurity ] Sensitive data recording failed:" + err.Error())
				return
			}

			//敏感字段
			dataFields := map[string]string{}
			err = json.Unmarshal([]byte(sensitive.DataFields), &dataFields)
			if err != nil {
				m.log.Warn("[ DataSecurity ] Sensitive data recording failed:" + err.Error())
				return
			}

			//创建修改记录
			admin := header.GetAdminAuth(c)
			sensitiveDataLogs := []model.SecuritySensitiveDataLog{}
			for k, v := range dataFields {
				beforeV, oldOk := row[k]
				afterV, newOk := params[k]
				idValue := com.StrTo(fmt.Sprintf("%v", params["id"])).MustInt()

				if oldOk && newOk && beforeV != afterV {
					sensitiveDataLogs = append(sensitiveDataLogs, model.SecuritySensitiveDataLog{
						AdminID:     admin.Id,
						SensitiveID: sensitive.ID,
						DataTable:   sensitive.DataTable,
						PrimaryKey:  sensitive.PrimaryKey,
						DataField:   k,
						DataComment: v,
						IDValue:     int32(idValue),
						Before:      fmt.Sprintf("%v", beforeV),
						After:       fmt.Sprintf("%v", afterV),
						IP:          c.ClientIP(),
						Useragent:   c.Request.Header.Get("User-Agent"),
					})
				}
			}
			if len(sensitiveDataLogs) == 0 {
				return
			}

			err = m.sqlDB.Model(&model.SecuritySensitiveDataLog{}).Create(&sensitiveDataLogs).Error
			if err != nil {
				m.log.Warn("[ DataSecurity ] Sensitive data recording failed:" + err.Error())
				return
			}
			return
		}
	}
}
