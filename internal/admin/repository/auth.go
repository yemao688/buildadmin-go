package repository

import (
	"errors"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/header"
	passwordutil "buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/permissioncache"
	"buildadmin-go/internal/pkg/random"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"buildadmin-go/internal/pkg/token"
	"buildadmin-go/internal/utils"
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
// current administrator's permissions.
func (s *AuthRepository) GetAllRuleNames() ([]string, error) {
	if s == nil || s.sqlDB == nil {
		return nil, errors.New("authorization database is unavailable")
	}
	return s.cache.AllRuleNames(func() ([]string, error) {
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
	})
}

func (s *AuthRepository) IsLogin(ctx *gin.Context) (*token.Token, bool) {
	tokenStr := ctx.Request.Header.Get("batoken")
	if tokenStr == "" {
		tokenStr = ctx.Query("batoken")
	}
	if tokenStr != "" {
		tokenData, err := s.tokenHelper.GetFor(tokenStr, "admin")
		if err == nil {
			return tokenData, true
		}
	}
	return nil, false
}

func (s *AuthRepository) IsEnabledAdmin(id int32) bool {
	var admin model.Admin
	err := s.sqlDB.Model(&model.Admin{}).Select("status").Where("id=?", id).First(&admin).Error
	return err == nil && admin.Status == "enable"
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

func (s *AuthRepository) Login(ctx *gin.Context, username string, password string, keep bool) (interface{}, error) {
	admin := model.Admin{}
	err := s.sqlDB.Model(&model.Admin{}).Where("username=?", username).Scan(&admin).Error
	if err != nil {
		return nil, cErr.BadRequest("Incorrect user name or password!")
	}

	if !utils.AccountStatusEnabled(admin.Status) {
		return nil, cErr.BadRequest("Username is incorrect")
	}

	retry := s.config.App.AdminLoginRetry
	if retry > 0 && admin.LoginFailure >= int32(retry) && (time.Now().Unix()-admin.LastLoginTime < 86400) {
		return nil, cErr.BadRequest("Please try again after 1 day")
	}

	if err := passwordutil.Compare(admin.Password, password); err != nil {
		s.sqlDB.Model(&model.Admin{}).Where("id=?", admin.ID).Updates(map[string]interface{}{
			"login_failure":   admin.LoginFailure + 1,
			"last_login_time": time.Now().Unix(),
			"last_login_ip":   ctx.ClientIP(),
		})
		return nil, cErr.BadRequest("Password is incorrect")
	}

	if s.config.App.AdminSso {
		s.tokenHelper.Clear("admin", admin.ID)
		s.tokenHelper.Clear("admin-refresh", admin.ID)
	}

	refreshToken := ""
	if keep {
		refreshToken = random.Uuid()
		s.tokenHelper.Set(refreshToken, "admin-refresh", admin.ID, 2592000) //30天
	}
	token := random.Uuid()
	if err := s.tokenHelper.Set(token, "admin", admin.ID, s.config.App.AdminTokenKeepTime); err != nil {
		return nil, err
	}

	err = s.sqlDB.Model(&model.Admin{}).Where("id=?", admin.ID).Updates(map[string]interface{}{
		"login_failure":   0,
		"last_login_time": time.Now().Unix(),
		"last_login_ip":   ctx.ClientIP(),
	}).Error

	return map[string]interface{}{
		"id":              admin.ID,
		"username":        admin.Username,
		"nickname":        admin.Nickname,
		"avatar":          admin.Avatar,
		"last_login_time": time.Now().Unix(),
		"token":           token,
		"refresh_token":   refreshToken,
	}, err
}

func (s *AuthRepository) Logout(ctx *gin.Context, refreshToken string) error {
	if refreshToken != "" {
		if err := s.tokenHelper.Delete(refreshToken); err != nil {
			return err
		}
	}
	adminAuth := header.GetAdminAuth(ctx)
	if err := s.tokenHelper.Delete(adminAuth.Token); err != nil {
		return err
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
	ruleNameList := s.cache.RuleNames(id)
	if slices.Contains(ruleNameList, "*") {
		return true
	}
	result := false
	checkNameArr := strings.Split(strings.ToLower(name), ",")
	for _, v := range checkNameArr {
		if slices.Contains(ruleNameList, v) {
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
