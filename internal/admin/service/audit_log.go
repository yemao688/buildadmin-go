package service

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	adminrepo "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
)

var (
	auditLogSkipURLPattern      = regexp.MustCompile(`(?i)/(select|index|logout)$`)
	auditLogSensitiveKeyPattern = regexp.MustCompile(`(?i)(password|token)`)
)

const (
	auditLogURLMaxBytes       = 1500
	auditLogUserAgentMaxBytes = 255
	auditLogUnknownUsername   = "Unknown"
)

// AuditLogService 承载后台操作日志的写入编排：跳过列表/选择器/登出请求、
// 用户名回退、规则标题反查、敏感字段脱敏与字段截断。请求上下文采集留在
// middleware，落库只经 AdminLogRepository.Create。
type AuditLogService struct {
	adminLogM  *adminrepo.AdminLogRepository
	adminRuleM *adminrepo.AdminRuleRepository
	authM      *adminrepo.AuthRepository
}

func NewAuditLogService(adminLogM *adminrepo.AdminLogRepository, adminRuleM *adminrepo.AdminRuleRepository, authM *adminrepo.AuthRepository) *AuditLogService {
	return &AuditLogService{
		adminLogM:  adminLogM,
		adminRuleM: adminRuleM,
		authM:      authM,
	}
}

// Record 记录一条后台操作日志。actor/username/url/title/ip/userAgent 由
// middleware 从 gin 上下文提取后作为普通参数传入；本方法不做传输层解析。
func (s *AuditLogService) Record(ctx context.Context, actor data_scope.Actor, username, url, title, ip, userAgent string, params map[string]interface{}) error {
	if s == nil || s.adminLogM == nil || skipAuditLogURL(url) {
		return nil
	}
	adminLog := model.AdminLog{
		AdminID:   actor.AdminID,
		Username:  s.resolveUsername(ctx, actor, username, params),
		URL:       truncateAuditLogUTF8(url, auditLogURLMaxBytes),
		Title:     s.resolveTitle(ctx, actor, url, title),
		Data:      marshalAuditLogParams(params),
		IP:        ip,
		Useragent: truncateAuditLogUTF8(userAgent, auditLogUserAgentMaxBytes),
	}
	return s.adminLogM.Create(ctx, &adminLog)
}

func skipAuditLogURL(url string) bool {
	return auditLogSkipURLPattern.MatchString(url)
}

func sanitizeAuditLogValue(value interface{}) interface{} {
	switch value := value.(type) {
	case map[string]interface{}:
		for key, nested := range value {
			if auditLogSensitiveKeyPattern.MatchString(key) {
				value[key] = "***"
				continue
			}
			value[key] = sanitizeAuditLogValue(nested)
		}
	case []interface{}:
		for i, nested := range value {
			value[i] = sanitizeAuditLogValue(nested)
		}
	}
	return value
}

func truncateAuditLogUTF8(value string, maxBytes int) string {
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

func marshalAuditLogParams(params map[string]interface{}) string {
	if params == nil {
		params = map[string]interface{}{}
	}
	sanitizeAuditLogValue(params)
	data, err := json.Marshal(params)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// resolveUsername 按登录态用户名 → 管理员表 → 请求参数 → Unknown 的顺序回退，
// 保证历史日志总有一个可读的操作者标识。
func (s *AuditLogService) resolveUsername(ctx context.Context, actor data_scope.Actor, username string, params map[string]interface{}) string {
	if username != "" {
		return username
	}
	if s.authM != nil {
		if value, ok := s.authM.UsernameByID(ctx, actor.AdminID); ok {
			return value
		}
	}
	if value, ok := params["username"].(string); ok && value != "" {
		return value
	}
	return auditLogUnknownUsername
}

// resolveTitle 优先使用 handler 写入的 log_title；否则按 URL 反查规则标题，
// 只有父子规则标题都命中时才组装 "父-子"，否则回退 Unknown(action)。
func (s *AuditLogService) resolveTitle(ctx context.Context, actor data_scope.Actor, url, title string) string {
	if title != "" {
		return title
	}
	name := strings.Trim(strings.TrimPrefix(url, "/admin/"), "/")
	name = strings.ToLower(strings.ReplaceAll(name, ".", "/"))
	action := name
	if slashIndex := strings.LastIndex(name, "/"); slashIndex != -1 {
		action = name[slashIndex+1:]
	}
	actionTitle, actionOK := s.ruleTitle(ctx, actor, name)
	if slashIndex := strings.LastIndex(name, "/"); slashIndex != -1 {
		parentTitle, parentOK := s.ruleTitle(ctx, actor, name[:slashIndex])
		if actionOK && parentOK {
			return parentTitle + "-" + actionTitle
		}
	}
	return "Unknown(" + action + ")"
}

func (s *AuditLogService) ruleTitle(ctx context.Context, actor data_scope.Actor, name string) (string, bool) {
	if s.authM != nil && actor.AdminID != 0 {
		if title, ok := s.authM.CachedRuleTitle(actor.AdminID, name); ok {
			return title, true
		}
	}
	if s.adminRuleM == nil {
		return "", false
	}
	title, ok, err := s.adminRuleM.TitleByName(ctx, name)
	if err != nil {
		return "", false
	}
	return title, ok
}
