package model

import (
	"fmt"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-build-admin/app/pkg/token"
	"go-build-admin/utils"
)

var AuthGroupList map[int32][]AuthGroup = make(map[int32][]AuthGroup)
var AuthRuleList map[int32][]Rule = make(map[int32][]Rule)
var AuthRuleNameList map[int32][]string = make(map[int32][]string)

var muGroup sync.Mutex
var muRule sync.Mutex

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
	Children  []Rule `json:"children"`
}

type AuthModel struct {
	sqlDB       *gorm.DB
	tokenHelper *token.TokenHelper
	config      *conf.Configuration
}

func NewAuthModel(sqlDB *gorm.DB, tokenHelper *token.TokenHelper, config *conf.Configuration) *AuthModel {
	return &AuthModel{sqlDB: sqlDB, tokenHelper: tokenHelper, config: config}
}

func (s *AuthModel) IsLogin(ctx *gin.Context) (*token.Token, bool) {
	tokenStr := ctx.Request.Header.Get("Authorization")
	if tokenStr != "" {
		tokenData, err := s.tokenHelper.Get(tokenStr, true)
		if err != nil {
			return tokenData, true
		}
	}
	return nil, false
}

func (s *AuthModel) GetInfo(ctx *gin.Context, id int32) (Admin, error) {
	admin := Admin{}
	err := s.sqlDB.Table(TableNameAdmin).Where("id=?", id).Scan(&admin).Error
	return admin, err
}

func (s *AuthModel) IsSuperAdmin(id int32) bool {
	rules, err := s.GetRuleIds(id)
	if err != nil {
		return false
	}
	if utils.ContainsString(rules, "*") {
		return true
	}
	return false
}

func (s *AuthModel) Login(ctx *gin.Context, username string, password string, keep bool) (interface{}, error) {
	admin := Admin{}
	err := s.sqlDB.Table(TableNameAdmin).Where("username=?", username).Scan(&admin).Error
	if err != nil {
		return nil, cErr.BadRequest("Incorrect user name or password!")
	}

	if admin.Status == "0" {
		return nil, cErr.BadRequest("Username is incorrect")
	}

	retry := s.config.App.AdminLoginRetry
	if retry > 0 && admin.LoginFailure >= int32(retry) && (time.Now().Unix()-admin.LastLoginTime < 86400) {
		return nil, cErr.BadRequest("Please try again after 1 day")
	}

	if admin.Password != utils.EncryptPassword(password, admin.Salt) {
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
	if err := s.tokenHelper.Set(token, "admin", admin.ID, 86400); err != nil {
		return nil, err
	}

	err = s.sqlDB.Table(TableNameAdmin).Where("id=?", admin.ID).Updates(map[string]interface{}{
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

func (s *AuthModel) Logout(ctx *gin.Context, refreshToken string) error {
	if err := s.tokenHelper.Delete(refreshToken); err != nil {
		return err
	}
	adminAuth := header.GetAdminAuth(ctx)
	if err := s.tokenHelper.Delete(adminAuth.Token); err != nil {
		return err
	}
	return nil
}

// 获取菜单规则列表
func (s *AuthModel) GetMenus(ctx *gin.Context, id int32) (rules []Rule, err error) {
	fmt.Printf("%+v \n", 1111)
	if _, ok := AuthRuleList[id]; !ok {
		if _, err = s.GetRuleList(ctx, id); err != nil {
			return
		}
	}
	fmt.Printf("%+v \n", 222)
	if len(AuthRuleList[id]) == 0 {
		rules = []Rule{}
		return
	}
	fmt.Printf("%+v \n", 333)
	children := map[int32][]Rule{}
	for _, v := range AuthRuleList[id] {
		children[v.Pid] = append(children[v.Pid], v)
	}

	if len(children) == 0 {
		return
	}
	rules = s.getChildren(children, children[0])
	return
}

// 获取传递的菜单规则的子规则
func (s *AuthModel) getChildren(children map[int32][]Rule, rules []Rule) []Rule {
	for key, v := range rules {
		if _, ok := children[v.ID]; ok {
			rules[key].Children = s.getChildren(children, children[v.ID])
		}
	}
	return rules
}

// 检查是否有某权限
func (s *AuthModel) Check(name string, id int32, relation string) bool {
	ruleNameList := AuthRuleNameList[id]
	if utils.ContainsString(ruleNameList, "*") {
		return true
	}

	result := false
	checkNameArr := strings.Split(strings.ToLower(name), ",")
	for _, v := range ruleNameList {
		if utils.ContainsString(checkNameArr, v) {
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

// 获得权限规则列表
func (s *AuthModel) GetRuleList(ctx *gin.Context, id int32) ([]string, error) {
	muRule.Lock()
	defer muRule.Unlock()

	ids, err := s.GetRuleIds(id)

	fmt.Printf("%+v\n", ids)
	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		AuthRuleList[id] = []Rule{}
		return []string{}, nil
	}

	tx := s.sqlDB.Table("ba_admin_rule").Where("status=?", "1")
	if ok := utils.ContainsString(ids, "*"); !ok {
		tx.Where("id in ?", ids)
	}
	var ruleList []Rule
	tx.Order("weigh desc,id asc").Scan(&ruleList)

	ruleNameList := []string{}
	if ok := utils.ContainsString(ids, "*"); ok {
		ruleNameList = append(ruleNameList, "*")
	}

	seen := make(map[string]bool)
	for _, v := range ruleList {
		if _, ok := seen[v.Name]; !ok {
			seen[v.Name] = true
			ruleNameList = append(ruleNameList, v.Name)
		}
	}
	AuthRuleList[id] = ruleList
	AuthRuleNameList[id] = ruleNameList

	fmt.Printf("%+v \n", AuthRuleNameList)
	return ruleNameList, nil

}

// 获取权限规则ids
func (s *AuthModel) GetRuleIds(id int32) ([]string, error) {
	groups, err := s.GetGroups(id)
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
func (s *AuthModel) GetGroups(id int32) ([]AuthGroup, error) {
	muGroup.Lock()
	defer muGroup.Unlock()
	if val, ok := AuthGroupList[id]; ok {
		return val, nil
	}

	var authGroups []AuthGroup
	err := s.sqlDB.Table(TableNameAdminGroupAccess).
		Joins("left join ba_admin_group on ba_admin_group.id=ba_admin_group_access.group_id").
		Where("ba_admin_group_access.uid=? and ba_admin_group.status='1'", id).
		Scan(&authGroups).Error

	AuthGroupList[id] = authGroups
	return authGroups, err
}

// 获取管理员所在分组的所有子级分组
func (s *AuthModel) GetAdminChildGroups(id int32) []int32 {
	accessList := []AdminGroupAccess{}
	s.sqlDB.Table(TableNameAdminGroupAccess).Where("id=?", id).Find(&accessList)
	children := []int32{}
	for _, v := range accessList {
		children = append(children, s.GetGroupChildGroups(v.GroupID)...)
	}
	return children
}

// 获取一个分组下的子分组
func (s *AuthModel) GetGroupChildGroups(groupId int32) []int32 {
	adminGroups := []AdminGroup{}
	s.sqlDB.Table(TableNameAdminGroup).Where("pid=? and status=?", groupId, 1).Find(&adminGroups)
	children := []int32{}
	for _, v := range adminGroups {
		children = append(children, v.ID)
		subIds := s.GetGroupChildGroups(v.ID)
		children = append(children, subIds...)
	}
	return children
}

// 获取分组内的管理员
func (s *AuthModel) GetGroupAdmins(ids interface{}) []int32 {
	adminIds := []int32{}
	s.sqlDB.Table(TableNameAdminGroupAccess).Where("group_id in ?", ids).Pluck("uid", &adminIds)
	return adminIds
}

// 获取拥有"所有权限"的分组
func (s *AuthModel) GetAllAuthGroups(dataLimit string, id int32) ([]string, error) {
	rules, err := s.GetRuleIds(id)
	if err != nil {
		return nil, err
	}

	rulesStr := "," + strings.Join(rules, ",") + ","
	allAuthGroups := []string{}
	groups := []AdminGroup{}
	s.sqlDB.Table(TableNameAdminGroup).Where("status=1").Find(&groups)
	for _, v := range groups {
		if v.Rules == "*" {
			continue
		}

		groupRules := strings.Split(v.Rules, ",")
		all := true
		for _, r := range groupRules {
			if !strings.Contains(rulesStr, ","+r+",") {
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
func (s *AuthModel) GetGroupIds(id int32) []int32 {
	groupIds := []int32{}
	s.sqlDB.Table(TableNameAdminGroupAccess).Where("uid=?", id).Pluck("group_id", &groupIds)
	return groupIds
}
