package auth

import (
	"encoding/json"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/persistence"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var adminLogSkipURLPattern = regexp.MustCompile(`(?i)/(select|index|logout)$`)
var adminLogSensitiveKeyPattern = regexp.MustCompile(`(?i)(password|token)`)

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

type AdminLogRepository struct {
	persistence.BaseModel
	authM *AuthRepository
}

func NewAdminLogRepository(sqlDB *gorm.DB, config *conf.Configuration, authM *AuthRepository) *AdminLogRepository {
	return &AdminLogRepository{
		BaseModel: persistence.NewBaseModel(config.Database.Prefix+"admin_log", "id", "title", sqlDB),
		authM:     authM,
	}
}

// GetEnabledAdmin returns the username from the same status check used by the
// admin authentication middleware.
func (s *AuthRepository) GetEnabledAdmin(id int32) (string, bool) {
	if s == nil || s.sqlDB == nil {
		return "", false
	}
	var admin struct {
		Status   string
		Username string
	}
	err := s.sqlDB.Model(&model.Admin{}).Select("status, username").Where("id=?", id).First(&admin).Error
	return admin.Username, err == nil && admin.Status == "enable"
}

// CachedRuleTitle returns a title from the administrator's permission cache.
// It intentionally does not load the cache; authorization owns cache loading.
func (s *AuthRepository) CachedRuleTitle(uid int32, name string) (string, bool) {
	if s == nil {
		return "", false
	}
	rules, ok := s.cache.Rules(uid)
	if !ok {
		return "", false
	}
	name = strings.ToLower(name)
	for _, rule := range rules {
		if strings.ToLower(rule.Name) == name {
			return rule.Title, true
		}
	}
	return "", false
}

func (s *AdminLogRepository) List(ctx *gin.Context) (list []model.AdminLog, total int64, err error) {
	whereS, whereP, orderS, limit, offset, err := adminmodel.QueryBuilder(ctx, s.TableInfo(), nil)
	if err != nil {
		return nil, 0, err
	}
	db := s.DB().Model(&model.AdminLog{}).Scopes(adminmodel.IsSuperAdmin(ctx)).Where(whereS, whereP...)
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = db.Order(orderS).Limit(limit).Offset(offset).Find(&list).Error
	return
}

func (s *AdminLogRepository) Add(ctx *gin.Context, params map[string]interface{}) {
	if s == nil || s.DB() == nil || ctx == nil || ctx.Request == nil || ctx.Request.URL == nil {
		return
	}
	url := ctx.Request.URL.Path
	// 排除列表、选择器和登出请求；写入其它管理操作（包括 POST/DELETE）。
	if skipAdminLogURL(url) {
		return
	}

	info := header.GetAdminAuth(ctx)
	username := ""
	if info.Username != "" {
		username = info.Username
	} else if info.Id != 0 {
		admin := model.Admin{}
		s.DBFor(ctx).Where("id=?", info.Id).First(&admin)
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
		lookupRuleTitle := func(ruleName string) (string, bool) {
			if s.authM != nil && info.Id != 0 {
				if cachedTitle, ok := s.authM.CachedRuleTitle(info.Id, ruleName); ok {
					return cachedTitle, true
				}
			}
			rule := model.AdminRule{}
			s.DBFor(ctx).Where("name=?", ruleName).First(&rule)
			return rule.Title, rule.ID != 0
		}
		actionTitle, actionOK := lookupRuleTitle(name)

		slashIndex := strings.LastIndex(name, "/")
		if slashIndex != -1 {
			parentTitle, parentOK := lookupRuleTitle(name[:slashIndex])
			if actionOK && parentOK {
				title = parentTitle + "-" + actionTitle
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
	adminLog := model.AdminLog{
		AdminID:   info.Id,
		Username:  username,
		URL:       truncateAdminLogUTF8(url, 1500),
		Title:     title,
		Data:      string(data),
		IP:        ctx.ClientIP(),
		Useragent: truncateAdminLogUTF8(ctx.Request.UserAgent(), 255),
	}
	if err := s.DBFor(ctx).Create(&adminLog).Error; err != nil {
		zap.L().Warn("failed to create admin log", zap.Error(err))
	}
}

func (s *AdminLogRepository) Del(ctx *gin.Context, ids interface{}) error {
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
	return s.Transaction(ctx, func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.AdminLog{}).Where("id IN ?", normalized).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(normalized)) {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&model.AdminLog{}).Where("id IN ?", normalized).Delete(nil).Error
	})
}
