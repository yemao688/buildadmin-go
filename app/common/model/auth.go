package model

import (
	"fmt"
	"go-build-admin/app/admin/model"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/random"
	"go-build-admin/conf"
	"regexp"
	"slices"
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
	UID     int32  `json:"uid"`      // 用户ID
	GroupID int32  `json:"group_id"` // 分组ID
	ID      int32  `json:"id"`       // ID
	Name    string `json:"name"`     // 组名
	Rules   string `json:"rules"`    // 权限规则ID
}

type Rule struct {
	ID           int32  `json:"id"`             // ID
	Pid          int32  `json:"pid"`            // 上级菜单
	Type         string `json:"type"`           // 类型:menu_dir=菜单目录,menu=菜单项,button=页面按钮
	Title        string `json:"title"`          // 标题
	Name         string `json:"name"`           // 规则名称
	Path         string `json:"path"`           // 路由路径
	Icon         string `json:"icon"`           // 图标
	MenuType     string `json:"menu_type"`      // 菜单类型:tab=选项卡,link=链接,iframe=Iframe
	URL          string `json:"url"`            // Url
	Component    string `json:"component"`      // 组件路径
	NoLoginValid string `json:"no_login_valid"` // 未登录有效:0=否,1=是
	Extend       string `json:"extend"`         // 扩展属性:none=无,add_rules_only=只添加为路由,add_menu_only=只添加为菜单
	Children     []Rule `json:"children"`
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
	tokenStr := ctx.Request.Header.Get("ba-user-token")
	if tokenStr != "" {
		tokenData, err := s.tokenHelper.Get(tokenStr)
		if err == nil {
			return tokenData, true
		}
	}
	return nil, false
}

func (s *AuthModel) SetVerificationToken(t string, id int32) string {
	tokenStr := random.Uuid()
	s.tokenHelper.Set(tokenStr, t, id, 600) //30天
	return tokenStr
}

func (s *AuthModel) VerificationToken(token string, t string, user_id int32) bool {
	result := s.tokenHelper.Check(token, t, user_id)
	return result
}

func (s *AuthModel) DelVerificationToken(token string) {
	s.tokenHelper.Delete(token)
}

func (s *AuthModel) GetInfo(ctx *gin.Context, id int32) (User, error) {
	user := User{}
	err := s.sqlDB.Model(&User{}).Where("id=?", id).Scan(&user).Error
	return user, err
}

func (s *AuthModel) Login(ctx *gin.Context, username string, password string, keep bool) (interface{}, error) {
	// 判断账户类型
	accountType := ""
	phoneRegex := regexp.MustCompile(`^1[3-9]\d{9}$`)
	if phoneRegex.MatchString(username) {
		accountType = "mobile"
	}
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if emailRegex.MatchString(username) {
		accountType = "email"
	}

	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]{4,30}$`)
	if usernameRegex.MatchString(username) {
		accountType = "username"
	}

	if accountType == "" {
		return nil, cErr.BadRequest("Account not exist")
	}

	user := User{}
	err := s.sqlDB.Model(&User{}).Where(accountType+"=?", username).Scan(&user).Error
	if err != nil {
		return nil, cErr.BadRequest("Account not exist")
	}

	if !utils.AccountStatusEnabled(user.Status) {
		return nil, cErr.BadRequest("Account disabled")
	}

	retry := s.config.App.UserLoginRetry
	if retry > 0 && user.LoginFailure >= int32(retry) && (time.Now().Unix()-user.LastLoginTime < 86400) {
		return nil, cErr.BadRequest("Please try again after 1 day")
	}

	if user.Password != utils.EncryptPassword(password, user.Salt) {
		s.sqlDB.Model(&User{}).Where("id=?", user.ID).Updates(map[string]interface{}{
			"login_failure":   user.LoginFailure + 1,
			"last_login_time": time.Now().Unix(),
			"last_login_ip":   ctx.ClientIP(),
		})
		return nil, cErr.BadRequest("Password is incorrect")
	}

	if s.config.App.UserSso {
		s.tokenHelper.Clear("user", user.ID)
		s.tokenHelper.Clear("user-refresh", user.ID)
	}

	refreshToken := ""
	if keep {
		refreshToken = random.Uuid()
		s.tokenHelper.Set(refreshToken, "user-refresh", user.ID, 2592000) //30天
	}
	token := random.Uuid()
	if err := s.tokenHelper.Set(token, "user", user.ID, s.config.App.UserTokenKeepTime); err != nil {
		return nil, err
	}

	err = s.sqlDB.Model(&User{}).Where("id=?", user.ID).Updates(map[string]interface{}{
		"login_failure":   0,
		"last_login_time": time.Now().Unix(),
		"last_login_ip":   ctx.ClientIP(),
	}).Error

	userInfo := s.FilterData(user)
	userInfo["token"] = token
	userInfo["refresh_token"] = refreshToken
	return userInfo, err
}

func (s *AuthModel) FilterData(user User) map[string]any {

	birthday := ""
	if user.Birthday.Unix() > 100 {
		birthday = user.Birthday.Format("2006-01-02")
	}
	return map[string]any{
		"id":              user.ID,
		"username":        user.Username,
		"nickname":        user.Nickname,
		"email":           user.Email,
		"mobile":          user.Mobile,
		"avatar":          user.Avatar,
		"gender":          user.Gender,
		"birthday":        birthday,
		"money":           fmt.Sprintf("%.2f", float64(user.Money/100)),
		"score":           user.Score,
		"join_time":       user.JoinTime,
		"motto":           user.Motto,
		"last_login_time": time.Now().Unix(),
		"last_login_ip":   user.LastLoginIP,
	}
}

func (s *AuthModel) Register(ctx *gin.Context, username string, password string, mobile string, email string) (interface{}, error) {
	existUser := User{}
	if username != "" {
		err := s.sqlDB.Model(&User{}).Where("username=?", username).Scan(&existUser).Error
		if err != nil {
			return nil, cErr.BadRequest("Username is exist!")
		}
	}

	if email != "" {
		err := s.sqlDB.Model(&User{}).Where("email=?", email).Scan(&existUser).Error
		if err != nil {
			return nil, cErr.BadRequest("Email is exist!")
		}
	}

	if mobile != "" {
		err := s.sqlDB.Model(&User{}).Where("mobile=?", mobile).Scan(&existUser).Error
		if err != nil {
			return nil, cErr.BadRequest("Mobile is exist!")
		}
	}

	salt := random.Build("alnum", 16)
	password = utils.EncryptPassword(password, salt)

	nickname := utils.MaskPhone(username)
	user := User{
		GroupID:       1,
		Username:      username,
		Nickname:      nickname,
		Email:         email,
		Mobile:        mobile,
		Avatar:        "",
		Gender:        0,
		LastLoginTime: time.Now().Unix(),
		LastLoginIP:   ctx.ClientIP(),
		LoginFailure:  0,
		JoinIP:        ctx.ClientIP(),
		JoinTime:      time.Now().Unix(),
		Motto:         "",
		Password:      password,
		Salt:          salt,
		Status:        "enable",
	}

	if err := s.sqlDB.Create(&user).Error; err != nil {
		return nil, err
	}

	token := random.Uuid()
	if err := s.tokenHelper.Set(token, "user", user.ID, s.config.App.UserTokenKeepTime); err != nil {
		return nil, err
	}

	userInfo := s.FilterData(user)
	userInfo["token"] = token
	userInfo["refresh_token"] = ""
	return userInfo, nil
}

func (s *AuthModel) Logout(ctx *gin.Context, refreshToken string) error {
	if refreshToken != "" {
		if err := s.tokenHelper.Delete(refreshToken); err != nil {
			return err
		}
	}
	userAuth := header.GetUserAuth(ctx)
	if err := s.tokenHelper.Delete(userAuth.Token); err != nil {
		return err
	}
	return nil
}

// 获取菜单规则列表
func (s *AuthModel) GetMenus(ctx *gin.Context, uid int32) (rules []Rule, err error) {
	if _, ok := AuthRuleList[uid]; !ok {
		if _, err = s.GetRuleList(ctx, uid); err != nil {
			return
		}
	}
	if len(AuthRuleList[uid]) == 0 {
		rules = []Rule{}
		return
	}
	children := map[int32][]Rule{}
	for _, v := range AuthRuleList[uid] {
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

/**
 *检查是否有某权限
 *name  菜单规则的 name，可以传递两个，以','号隔开
 *uid   用户ID
 *relation 如果出现两个 name,是两个都通过(and)还是一个通过即可(or)
 */
func (s *AuthModel) Check(name string, id int32, relation string) bool {
	ruleNameList := AuthRuleNameList[id]
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

// 获得权限规则列表
func (s *AuthModel) GetRuleList(ctx *gin.Context, uid int32) ([]string, error) {
	muRule.Lock()
	defer muRule.Unlock()

	ids, err := s.GetRuleIds(uid)

	if err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		AuthRuleList[uid] = []Rule{}
		return []string{}, nil
	}

	tx := s.sqlDB.Model(&model.UserRule{}).Where("status=?", "1")
	if !slices.Contains(ids, "*") {
		tx.Where("id in ?", ids)
	}
	var ruleList []Rule
	tx.Order("weigh desc,id asc").Scan(&ruleList)

	ruleNameList := []string{}
	if slices.Contains(ids, "*") {
		ruleNameList = append(ruleNameList, "*")
	}

	seen := make(map[string]bool)
	for _, v := range ruleList {
		if _, ok := seen[v.Name]; !ok {
			seen[v.Name] = true
			ruleNameList = append(ruleNameList, v.Name)
		}
	}
	AuthRuleList[uid] = ruleList
	AuthRuleNameList[uid] = ruleNameList

	return ruleNameList, nil

}

// 获取权限规则ids
func (s *AuthModel) GetRuleIds(uid int32) ([]string, error) {
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
func (s *AuthModel) GetGroups(uid int32) ([]AuthGroup, error) {
	muGroup.Lock()
	defer muGroup.Unlock()
	if val, ok := AuthGroupList[uid]; ok {
		return val, nil
	}

	prefix := s.config.Database.Prefix
	var authGroups []AuthGroup
	err := s.sqlDB.Table(prefix+"user").
		Joins("left join "+prefix+"user_group on "+prefix+"user_group.id="+prefix+"user.group_id").
		Where(prefix+"user.id=? and "+prefix+"user_group.status='1'", uid).
		Scan(&authGroups).Error

	AuthGroupList[uid] = authGroups
	return authGroups, err
}
