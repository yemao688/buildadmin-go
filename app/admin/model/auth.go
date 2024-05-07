package model

import (
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"go-build-admin/app/pkg/token"
	"go-build-admin/utils"
)

var AuthGroupList map[int32][]AuthGroup
var AuthRuleList map[int32][]Rule
var AuthRuleNameList map[int32][]string

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
	Children  []Rule
}

type AuthInfo struct {
	ID            int32  `gorm:"column:id;primaryKey;autoIncrement:true;comment:ID" json:"id"` // ID
	Username      string `gorm:"column:username;not null;comment:用户名" json:"username"`         // 用户名
	Nickname      string `gorm:"column:nickname;not null;comment:昵称" json:"nickname"`          // 昵称
	Avatar        string `gorm:"column:avatar;not null;comment:头像" json:"avatar"`              // 头像
	LastLoginTime int64  `gorm:"column:last_login_time;comment:上次登录时间" json:"last_login_time"` // 上次登录时间
}

type AuthModel struct {
	sqlDB       *gorm.DB
	tokenHelper *token.TokenHelper
}

func NewAuthModel(sqlDB *gorm.DB, tokenHelper *token.TokenHelper) *AuthModel {
	return &AuthModel{sqlDB: sqlDB, tokenHelper: tokenHelper}
}

func (s *AuthModel) IsLogin(ctx *gin.Context) bool {

	return true
}

func (s *AuthModel) GetInfo(ctx *gin.Context, id int32) (list []Admin, err error) {

	return
}

func (s *AuthModel) IsSuperAdmin(ctx *gin.Context, id int32) bool {
	rules, err := s.GetRuleIds(ctx, id)
	if err != nil {
		return false
	}

	if utils.ContainsString(rules, "*") {
		return true
	}
	return false
}

func (s *AuthModel) Logout(ctx *gin.Context, refreshToken string) error {
	if err := s.tokenHelper.Delete(refreshToken); err != nil {
		return err
	}

	if err := s.tokenHelper.Delete(ctx.Keys["token"].(string)); err != nil {
		return err
	}
	return nil
}

// 获取菜单规则列表
func (s *AuthModel) GetMenus(ctx *gin.Context, id int32) (rules []Rule, err error) {
	if _, ok := AuthRuleList[id]; !ok {
		s.GetRuleList(ctx, id)
	}

	if len(AuthRuleList[id]) == 0 {
		return
	}

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

	if val, ok := AuthRuleNameList[id]; ok {
		return val, nil
	}
	ids, err := s.GetRuleIds(ctx, id)
	if err != nil {
		return nil, err
	}

	tx := s.sqlDB.Table("ba_admin_rule").Where("status=1")
	if ok := utils.ContainsString(ids, "*"); ok {
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
	return ruleNameList, nil

}

// 获取权限规则ids
func (s *AuthModel) GetRuleIds(ctx *gin.Context, id int32) ([]string, error) {
	groups, err := s.GetGroups(ctx, id)
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
func (s *AuthModel) GetGroups(ctx *gin.Context, id int32) ([]AuthGroup, error) {
	muGroup.Lock()
	defer muGroup.Unlock()

	if val, ok := AuthGroupList[id]; ok {
		return val, nil
	}
	var authGroups []AuthGroup
	err := s.sqlDB.Table("ba_admin_group_access").
		Joins("left join ba_admin_group on ba_admin_group.id=ba_admin_group_access.group_id").
		Where("ba_admin_group_access.uid=? and ba_admin_group.status='1'", id).
		Find(&authGroups).Error

	AuthGroupList[id] = authGroups
	return authGroups, err
}
