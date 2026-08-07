package repository

import (
	"errors"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/permissioncache"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"buildadmin-go/internal/pkg/token"
)

type AuthGroup struct {
	UID     int32  `json:"uid"`      // 管理员ID
	GroupID int32  `json:"group_id"` // 分组ID
	ID      int32  `json:"id"`       // ID
	Pid     int32  `json:"pid"`      // 上级分组
	Name    string `json:"name"`     // 组名
	Rules   string `json:"rules"`    // 权限规则ID
}

type Rule struct {
	ID        int32  `json:"id"`        // ID
	Pid       int32  `json:"pid"`       // 上级菜单
	Type      string `json:"type"`      // 类型:menu_dir=菜单目录,menu=菜单项,button=页面按钮
	Title     string `json:"title"`     // 标题
	Name      string `json:"name"`      // 规则名称
	Path      string `json:"path"`      // 路由路径
	Icon      string `json:"icon"`      // 图标
	MenuType  string `json:"menu_type"` // 菜单类型:tab=选项卡,link=链接,iframe=Iframe
	URL       string `json:"url"`       // Url
	Component string `json:"component"` // 组件路径
	Keepalive string `json:"keepalive"` // 缓存:0=关闭,1=开启
	Extend    string `json:"extend"`    // 扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单
	Children  []Rule `json:"children" gorm:"-"`
}

type AuthRepository struct {
	sqlDB       *gorm.DB
	tokenHelper *token.TokenHelper
	config      *conf.Configuration
	cache       permissioncache.Cache[AuthGroup, Rule]
}

func NewAuthRepository(sqlDB *gorm.DB, tokenHelper *token.TokenHelper, config *conf.Configuration) *AuthRepository {
	return &AuthRepository{
		sqlDB:       sqlDB,
		tokenHelper: tokenHelper,
		config:      config,
	}
}

// InvalidateUser clears the cached permissions for one administrator.
func (s *AuthRepository) InvalidateUser(uid int32) {
	s.cache.InvalidateUser(uid)
}

// InvalidateAll clears all cached administrator permissions.
func (s *AuthRepository) InvalidateAll() {
	s.cache.InvalidateAll()
}

// DatabaseAvailable checks the database before diagnostic startup work uses it.
func (s *AuthRepository) DatabaseAvailable() error {
	if s == nil || s.sqlDB == nil {
		return errors.New("authorization database is unavailable")
	}

	db, err := s.sqlDB.DB()
	if err != nil {
		return err
	}
	if db == nil {
		return errors.New("authorization database is unavailable")
	}
	if err := db.Ping(); err != nil {
		return err
	}
	return nil
}

// GetAllRuleNames returns the names of every admin rule, independent of the
// current administrator's permissions. The returned slice is read-only:
// callers must not modify it.
func (s *AuthRepository) GetAllRuleNames() ([]string, error) {
	if s == nil || s.sqlDB == nil {
		return nil, errors.New("authorization database is unavailable")
	}
	return s.cache.AllRuleNames(s.loadAllRuleNames)
}

// HasRuleName reports whether name is a registered admin rule, independent of
// the current administrator's permissions. The lookup is exact against the
// normalized (lowercased, deduplicated) rule-name set, byte-identical to the
// former GetAllRuleNames + slices.Contains membership check.
func (s *AuthRepository) HasRuleName(name string) (bool, error) {
	if s == nil || s.sqlDB == nil {
		return false, errors.New("authorization database is unavailable")
	}
	set, err := s.cache.AllNamesSet(s.loadAllRuleNames)
	if err != nil {
		return false, err
	}
	_, ok := set[name]
	return ok, nil
}

// loadAllRuleNames loads and normalizes every admin rule name: lowercased and
// deduplicated. The returned slice is stored by the cache as-is.
func (s *AuthRepository) loadAllRuleNames() ([]string, error) {
	var names []string
	if err := s.sqlDB.Model(&model.AdminRule{}).Pluck("name", &names).Error; err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(names))
	unique := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.ToLower(name)
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		unique = append(unique, name)
	}
	return unique, nil
}

func (s *AuthRepository) IsLogin(ctx *gin.Context) (*token.Token, bool) {
	tokenStr := ctx.Request.Header.Get("batoken")
	if tokenStr == "" {
		tokenStr = ctx.Query("batoken")
	}
	return s.TokenInfo(tokenStr)
}

// TokenInfo resolves an admin session token without touching the request.
// It is the transport-free counterpart of IsLogin for the service layer.
func (s *AuthRepository) TokenInfo(tokenStr string) (*token.Token, bool) {
	if tokenStr == "" {
		return nil, false
	}
	tokenData, err := s.tokenHelper.GetFor(tokenStr, "admin")
	if err == nil {
		return tokenData, true
	}
	return nil, false
}

func (s *AuthRepository) IsEnabledAdmin(id int32) bool {
	var admin model.Admin
	err := s.sqlDB.Model(&model.Admin{}).Select("status").Where("id=?", id).First(&admin).Error
	return err == nil && admin.Status == "enable"
}

// HasClosureSelfRow reports whether the administrator has the mandatory
// closure self-row (ancestor_id = descendant_id = adminID, depth=0). Every
// admin is expected to have one: the framework migration backfills and
// enforces them, and LinkNewNode inserts one per new admin. The login
// middleware performs this check once per request so the data-scope enforcer
// can drop the per-query self-EXISTS subquery. A missing self-row must deny
// scope, never grant it; the enforcer keeps the inline guard for that case.
func (s *AuthRepository) HasClosureSelfRow(adminID int32) (bool, error) {
	var count int64
	err := s.sqlDB.Table(s.config.Database.Prefix+"admin_closure").
		Where("ancestor_id = ? AND descendant_id = ?", adminID, adminID).
		Count(&count).Error
	return count > 0, err
}

func (s *AuthRepository) GetInfo(ctx *gin.Context, id int32) (model.Admin, error) {
	admin := model.Admin{}
	err := s.sqlDB.Model(&model.Admin{}).Where("id=?", id).Scan(&admin).Error
	return admin, err
}

func (s *AuthRepository) IsSuperAdmin(id int32) bool {
	rules, err := s.GetRuleIds(id)
	if err != nil {
		return false
	}
	if slices.Contains(rules, "*") {
		return true
	}
	return false
}

// GetByUsername scans the admin row by username. Scan semantics: ID == 0
// means the account does not exist (no RecordNotFound error).
func (s *AuthRepository) GetByUsername(username string) (model.Admin, error) {
	var admin model.Admin
	err := s.sqlDB.Model(&model.Admin{}).Where("username=?", username).Scan(&admin).Error
	return admin, err
}

// IncrementLoginFailure atomically bumps the failure counter and stamps the
// attempt time/ip. Concurrent failures cannot overwrite each other and the
// counter stays monotonic.
func (s *AuthRepository) IncrementLoginFailure(adminID int32, now int64, clientIP string) error {
	return s.sqlDB.Model(&model.Admin{}).Where("id=?", adminID).Updates(map[string]interface{}{
		"login_failure":   gorm.Expr("login_failure + 1"),
		"last_login_time": now,
		"last_login_ip":   clientIP,
	}).Error
}

// ResetLoginMeta clears the failure counter and stamps a successful login.
func (s *AuthRepository) ResetLoginMeta(adminID int32, now int64, clientIP string) error {
	return s.sqlDB.Model(&model.Admin{}).Where("id=?", adminID).Updates(map[string]interface{}{
		"login_failure":   0,
		"last_login_time": now,
		"last_login_ip":   clientIP,
	}).Error
}

// LockLoginState applies the login-throttle lock protocol under a row lock.
// An expired lockout window clears the failure counter before counting again
// (mirrors the member flow), and concurrent attempts serialize on the same
// row so the counter cannot be raced past the threshold. It is a
// self-contained atomic lock primitive: the service owns the throttle
// decision and the token issuance around it.
func (s *AuthRepository) LockLoginState(adminID int32, now int64, retry int) (bool, error) {
	if retry <= 0 {
		return false, nil
	}
	var state model.Admin
	locked := false
	err := s.sqlDB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Model(&model.Admin{}).
			Select("login_failure", "last_login_time").
			Where("id = ?", adminID).First(&state).Error; err != nil {
			return err
		}
		if state.LoginFailure <= 0 || state.LastLoginTime <= 0 {
			return nil
		}
		if now-state.LastLoginTime >= 86400 {
			// Expired window: clear the stale counter so one old failure
			// cannot re-lock the account for another day.
			return tx.Model(&model.Admin{}).Where("id = ?", adminID).Update("login_failure", 0).Error
		}
		locked = state.LoginFailure >= int32(retry)
		return nil
	})
	if err != nil {
		return false, err
	}
	return locked, nil
}

// Logout invalidates the refresh token and the caller's access token.
func (s *AuthRepository) Logout(refreshToken string, accessToken string) error {
	if refreshToken != "" {
		if err := s.tokenHelper.Delete(refreshToken); err != nil {
			return err
		}
	}
	if accessToken != "" {
		if err := s.tokenHelper.Delete(accessToken); err != nil {
			return err
		}
	}
	return nil
}

// 获取菜单规则列表
func (s *AuthRepository) GetMenus(ctx *gin.Context, uid int32) (rules []Rule, err error) {
	ruleList, ok := s.cachedRules(uid)
	if !ok {
		if _, err = s.GetRuleList(ctx, uid); err != nil {
			return
		}
		ruleList, _ = s.cachedRules(uid)
	}
	if len(ruleList) == 0 {
		rules = []Rule{}
		return
	}
	children := map[int32][]Rule{}
	for _, v := range ruleList {
		children[v.Pid] = append(children[v.Pid], v)
	}

	if len(children) == 0 {
		return
	}
	rules = s.getChildren(children, children[0])
	return
}

// 获取传递的菜单规则的子规则
func (s *AuthRepository) getChildren(children map[int32][]Rule, rules []Rule) []Rule {
	for key, v := range rules {
		if _, ok := children[v.ID]; ok {
			rules[key].Children = s.getChildren(children, children[v.ID])
		}
	}
	return rules
}

/**
 *检查是否有某权限
 *name  菜单规则的 name，可以传递两个，以','号隔开
 *uid   用户ID
 *relation 如果出现两个 name,是两个都通过(and)还是一个通过即可(or)
 */
func (s *AuthRepository) Check(name string, id int32, relation string) bool {
	ruleNameSet := s.cache.RuleNamesSet(id)
	if _, ok := ruleNameSet["*"]; ok {
		return true
	}
	result := false
	checkNameArr := strings.Split(strings.ToLower(name), ",")
	for _, v := range checkNameArr {
		if _, ok := ruleNameSet[v]; ok {
			result = true
		}

		if relation == "or" && result {
			break
		}

		if relation == "and" && !result {
			break
		}
	}
	return result
}

// EnsureRuleList loads an administrator's rules when the permission cache is
// cold. Check intentionally remains a cache-only operation.
func (s *AuthRepository) EnsureRuleList(ctx *gin.Context, uid int32) error {
	if _, ok := s.cachedRules(uid); ok {
		return nil
	}
	_, err := s.GetRuleList(ctx, uid)
	return err
}

// 获得权限规则列表
func (s *AuthRepository) GetRuleList(ctx *gin.Context, uid int32) ([]string, error) {
	return s.cache.ReloadRules(uid, func() ([]Rule, []string, error) {
		ids, err := s.GetRuleIds(uid)
		if err != nil {
			return nil, nil, err
		}
		if len(ids) == 0 {
			return []Rule{}, []string{}, nil
		}

		tx := s.sqlDB.Model(&model.AdminRule{}).Where("status=?", "1")
		if !slices.Contains(ids, "*") {
			tx = tx.Where("id in ?", ids)
		}
		var ruleList []Rule
		tx.Order("weigh desc,id asc").Scan(&ruleList)

		ruleNameList := []string{}
		if slices.Contains(ids, "*") {
			ruleNameList = append(ruleNameList, "*")
		}

		seen := make(map[string]bool)
		for _, v := range ruleList {
			name := strings.ToLower(v.Name)
			if _, ok := seen[name]; !ok {
				seen[name] = true
				ruleNameList = append(ruleNameList, name)
			}
		}
		return ruleList, ruleNameList, nil
	})
}

// 获取权限规则ids
func (s *AuthRepository) GetRuleIds(uid int32) ([]string, error) {
	groups, err := s.GetGroups(uid)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var result []string
	for _, v := range groups {
		strList := strings.Split(v.Rules, ",")
		for _, strItem := range strList {
			if _, ok := seen[strItem]; !ok {
				seen[strItem] = true
				result = append(result, strItem)
			}
		}
	}
	return result, nil
}

// 获取用户所有分组和对应权限规则
func (s *AuthRepository) GetGroups(uid int32) ([]AuthGroup, error) {
	return s.cache.GetOrLoadGroups(uid, func() ([]AuthGroup, error) {
		prefix := s.config.Database.Prefix
		var authGroups []AuthGroup
		err := s.sqlDB.Table(prefix+"admin_group_access").
			Joins("left join "+prefix+"admin_group on "+prefix+"admin_group.id="+prefix+"admin_group_access.group_id").
			Where(prefix+"admin_group_access.uid=? and "+prefix+"admin_group.status='1'", uid).
			Scan(&authGroups).Error
		return authGroups, err
	})
}

func (s *AuthRepository) cachedRules(uid int32) ([]Rule, bool) {
	return s.cache.Rules(uid)
}

// 获取拥有"所有权限"的分组
func (s *AuthRepository) GetAllAuthGroups(dataLimit string, id int32) ([]string, error) {
	rules, err := s.GetRuleIds(id)
	if err != nil {
		return nil, err
	}

	allAuthGroups := []string{}
	groups := []model.AdminGroup{}
	s.sqlDB.Model(&model.AdminGroup{}).Where("status=1").Find(&groups)
	for _, v := range groups {
		if v.Rules == "*" {
			continue
		}

		groupRules := strings.Split(v.Rules, ",")
		all := true
		for _, r := range groupRules {
			if !slices.Contains(rules, r) {
				all = false
				break
			}
		}
		if all {
			if dataLimit == "allAuth" || (dataLimit == "allAuthAndOthers" && len(rules) > len(groupRules)) {
				allAuthGroups = append(allAuthGroups, strconv.Itoa(int(v.ID)))
			}
		}
	}
	return allAuthGroups, nil
}

// 获取管理员的所在分组id
func (s *AuthRepository) GetGroupIds(id int32) []int32 {
	groupIds := []int32{}
	s.sqlDB.Model(&model.AdminGroupAccess{}).Where("uid=?", id).Pluck("group_id", &groupIds)
	return groupIds
}
