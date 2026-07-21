package model

import (
	"encoding/json"
	"go-build-admin/app/pkg/header"
	"go-build-admin/conf"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var adminLogSkipURLPattern = regexp.MustCompile(`(?i)/(select|index|logout)$`)
var adminLogSensitiveKeyPattern = regexp.MustCompile(`(?i)(password|salt|token)`)

func skipAdminLogURL(url string) bool {
	return adminLogSkipURLPattern.MatchString(url)
}

func sanitizeAdminLogValue(value interface{}) interface{} {
	switch value := value.(type) {
	case map[string]interface{}:
		for key, nested := range value {
			if adminLogSensitiveKeyPattern.MatchString(key) {
				value[key] = "***"
				continue
			}
			value[key] = sanitizeAdminLogValue(nested)
		}
	case []interface{}:
		for i, nested := range value {
			value[i] = sanitizeAdminLogValue(nested)
		}
	}
	return value
}

func truncateAdminLogUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	end := 0
	for index := range value {
		if index > maxBytes {
			break
		}
		end = index
	}
	return value[:end]
}

type AdminLog struct {
	ID         int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"`                     // ID
	AdminID    int32  `gorm:"column:admin_id;not null;comment:管理员ID" json:"admin_id"`                           // 管理员ID
	Username   string `gorm:"column:username;not null;comment:管理员用户名" json:"username"`                          // 管理员用户名
	URL        string `gorm:"column:url;not null;comment:操作Url" json:"url"`                                     // 操作Url
	Title      string `gorm:"column:title;not null;comment:日志标题" json:"title"`                                  // 日志标题
	Data       string `gorm:"column:data;comment:请求数据" json:"data"`                                             // 请求数据
	IP         string `gorm:"column:ip;not null;comment:IP" json:"ip"`                                          // IP
	Useragent  string `gorm:"column:useragent;not null;comment:User-Agent" json:"useragent"`                    // User-Agent
	CreateTime int64  `gorm:"autoCreateTime;column:create_time;autoCreateTime;comment:创建时间" json:"create_time"` // 创建时间
}

type AdminLogModel struct {
	BaseModel
}

func NewAdminLogModel(sqlDB *gorm.DB, config *conf.Configuration) *AdminLogModel {
	return &AdminLogModel{
		BaseModel: BaseModel{
			TableName:        config.Database.Prefix + "admin_log",
			Key:              "id",
			QuickSearchField: "title",
			sqlDB:            sqlDB,
		},
	}
}

func (s *AdminLogModel) List(ctx *gin.Context) (list []AdminLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.sqlDB.Model(&AdminLog{}).Scopes(IsSuperAdmin(ctx)).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *AdminLogModel) Add(ctx *gin.Context, params map[string]interface{}) {
	url := ctx.Request.URL.Path
	// 排除列表、选择器和登出请求；写入其它管理操作（包括 POST/DELETE）。
	if skipAdminLogURL(url) {
		return
	}

	info := header.GetAdminAuth(ctx)
	username := ""
	if info.Id != 0 {
		admin := Admin{}
		s.sqlDB.Where("id=?", info.Id).First(&admin)
		username = admin.Username
	} else if value, ok := params["username"].(string); ok && value != "" {
		username = value
	} else {
		username = "Unknown"
	}
	title := ctx.GetString("log_title")
	if title == "" {
		name := strings.Trim(strings.TrimPrefix(url, "/admin/"), "/")
		name = strings.ToLower(strings.ReplaceAll(name, ".", "/"))
		action := name
		if slashIndex := strings.LastIndex(name, "/"); slashIndex != -1 {
			action = name[slashIndex+1:]
		}
		actionRule := AdminRule{}
		s.sqlDB.Where("name=?", name).First(&actionRule)

		slashIndex := strings.LastIndex(name, "/")
		if slashIndex != -1 {
			parentRule := AdminRule{}
			s.sqlDB.Where("name=?", name[:slashIndex]).First(&parentRule)
			if actionRule.ID != 0 && parentRule.ID != 0 {
				title = parentRule.Title + "-" + actionRule.Title
			}
		}
		if title == "" {
			title = "Unknown(" + action + ")"
		}
	}

	if params == nil {
		params = map[string]interface{}{}
	}
	sanitizeAdminLogValue(params)
	data, err := json.Marshal(params)
	if err != nil {
		data = []byte("{}")
	}
	adminLog := AdminLog{
		AdminID:   info.Id,
		Username:  username,
		URL:       truncateAdminLogUTF8(url, 1500),
		Title:     title,
		Data:      string(data),
		IP:        ctx.ClientIP(),
		Useragent: truncateAdminLogUTF8(ctx.Request.UserAgent(), 255),
	}
	s.sqlDB.Create(&adminLog)
}

func (s *AdminLogModel) Del(ctx *gin.Context, ids interface{}) error {
	values, ok := ids.([]int32)
	if !ok || len(values) == 0 {
		return gorm.ErrInvalidData
	}
	seen := make(map[int32]struct{}, len(values))
	normalized := make([]int32, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			return gorm.ErrInvalidData
		}
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			normalized = append(normalized, id)
		}
	}
	return s.sqlDB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&AdminLog{}).Where("id IN ?", normalized).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&AdminLog{}).Where("id IN ?", normalized).Delete(nil).Error
	})
}
