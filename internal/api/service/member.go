package service

import (
	"buildadmin-go/internal/api/repository"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/random"
	"buildadmin-go/internal/pkg/systemroot"
	"buildadmin-go/internal/pkg/token"
	"fmt"
	"regexp"
	"sync"
	"time"

	"gorm.io/gorm"

	"buildadmin-go/internal/pkg/util"
)

type MemberService struct {
	users        *repository.UserRepository
	tokenHelper  *token.TokenHelper
	config       *conf.Configuration
	loginMetaTTL *loginMetaThrottle
}

var (
	loginPhoneRegex    = regexp.MustCompile(`^1[3-9]\d{9}$`)
	loginEmailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	loginUsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{4,30}$`)
)

func NewMemberService(sqlDB *gorm.DB, tokenHelper *token.TokenHelper, config *conf.Configuration) *MemberService {
	return &MemberService{
		users:        repository.NewUserRepository(sqlDB),
		tokenHelper:  tokenHelper,
		config:       config,
		loginMetaTTL: newLoginMetaThrottle(loginMetaWriteTTL),
	}
}

// loginMetaWriteTTL 是 ValidateUserToken 登录元信息写入的节流窗口：同一会员
// 在窗口内至多写一次 login_failure/last_login_time/last_login_ip。PHP 上游
// 只在登录时写，v3.x 改为每个已认证请求写一次库；窗口内跳过中间请求的写库，
// 最后一次登录时间精度随之降至至多 60s 一次（真实登录仍每次都写）。
const loginMetaWriteTTL = 60 * time.Second

// loginMetaThrottle 是进程内登录元信息写入节流器（先例：data_scope 的
// businessIdentifierCache 5 分钟 TTL 进程内缓存模式）。并发安全：读写均持锁；
// TTL 以请求上下文时间（now 注入，默认 time.Now）计算。只节流写频率，不缓存
// 任何账号状态，登出/禁用无需联动失效。
type loginMetaThrottle struct {
	mu      sync.Mutex
	entries map[int32]time.Time // uid → 最近一次写入时刻
	ttl     time.Duration
	now     func() time.Time // 可注入时钟（测试入口，不导出）
}

func newLoginMetaThrottle(ttl time.Duration) *loginMetaThrottle {
	return &loginMetaThrottle{
		entries: make(map[int32]time.Time),
		ttl:     ttl,
		now:     time.Now,
	}
}

// shouldWrite 报告该 uid 是否应执行一次登录元信息写入：TTL 窗口内已写过则
// 跳过（不更新写时刻，写时刻仍为窗口起点）；否则记录本次写入时刻并返回 true。
// 过期条目在再次访问时被覆盖，map 规模与曾出现过的 uid 数成正比。
func (t *loginMetaThrottle) shouldWrite(uid int32) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	if last, ok := t.entries[uid]; ok && now.Sub(last) < t.ttl {
		return false
	}
	t.entries[uid] = now
	return true
}

// IsLoginToken resolves a user session token. The header extraction from the
// request is a transport concern handled by the caller.
func (s *MemberService) IsLoginToken(tokenStr string) (*token.Token, bool) {
	if tokenStr == "" {
		return nil, false
	}
	tokenData, err := s.tokenHelper.GetFor(tokenStr, "user")
	if err == nil {
		return tokenData, true
	}
	return nil, false
}

func (s *MemberService) RefreshUserAccessToken(refreshToken string) (string, error) {
	initial, err := s.tokenHelper.Get(refreshToken)
	if err != nil {
		return "", err
	}
	if initial.Type != "user-refresh" {
		return "", cErr.BadRequest("Invalid token")
	}

	newToken := ""
	err = s.users.Transaction(func(tx *gorm.DB) error {
		user, err := s.users.LockByID(tx, initial.UserID)
		if err != nil {
			return err
		}
		if !util.AccountStatusEnabled(user.Status) {
			return cErr.BadRequest("Account disabled")
		}

		current, err := s.tokenHelper.Get(refreshToken)
		if err != nil {
			return err
		}
		if current.Type != "user-refresh" || current.UserID != initial.UserID {
			return cErr.BadRequest("Invalid token")
		}

		newToken = random.Uuid()
		return s.tokenHelper.Set(newToken, "user", user.ID, s.config.App.UserTokenKeepTime)
	})
	if err != nil {
		return "", err
	}
	return newToken, nil
}

// ValidateUserToken 校验会员访问 token 对应的账号并刷新登录元信息。
// 单次查询完成原 IsEnabledUser + GetByID 两步独立查库（v3.x 曾对每个已认证
// 请求依次执行 SELECT status 与全行 SELECT）：账号不存在、被禁用或加载失败
// 时返回与旧 IsEnabledUser 分支一致的 Unauthorized("Please login first")，
// 由 AbortLogin 渲染为 code=401 "Please login first"。登录元信息写入受
// loginMetaWriteTTL 节流窗口限制，窗口内跳过中间请求的写库；真实登录
// （Login/Register）不经过此处，仍每次写库。
func (s *MemberService) ValidateUserToken(id int32, ip string) error {
	user, err := s.users.GetByID(id)
	if err != nil {
		return err
	}
	if user == nil {
		return cErr.Unauthorized("Please login first")
	}
	if user.Status != "enable" {
		return cErr.Unauthorized("Please login first")
	}
	if !s.loginMetaTTL.shouldWrite(id) {
		return nil
	}
	return s.users.UpdateLoginMeta(id, 0, time.Now().Unix(), ip)
}

// Login verifies the credentials and issues the member tokens. ip is passed
// explicitly so the flow stays transport-free.
func (s *MemberService) Login(ip string, username string, plainPassword string, keep bool) (interface{}, error) {
	accountType := ""
	if loginPhoneRegex.MatchString(username) {
		accountType = "mobile"
	} else if loginEmailRegex.MatchString(username) {
		accountType = "email"
	} else if loginUsernameRegex.MatchString(username) {
		accountType = "username"
	}
	if accountType == "" {
		return nil, cErr.BadRequest("Account not exist")
	}

	user, err := s.users.GetByAccount(accountType, username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, cErr.BadRequest("Account not exist")
	}
	if !util.AccountStatusEnabled(user.Status) {
		return nil, cErr.BadRequest("Account disabled")
	}

	retry := s.config.App.UserLoginRetry
	if retry > 0 && user.LastLoginTime > 0 {
		now := time.Now().Unix()
		if user.LoginFailure > 0 && now-user.LastLoginTime >= 86400 {
			if err := s.users.ResetLoginFailure(user.ID); err != nil {
				return nil, err
			}
			user, err = s.users.GetByAccount(accountType, username)
			if err != nil {
				return nil, err
			}
			if user == nil {
				return nil, cErr.BadRequest("Account not exist")
			}
		}
		if user.LoginFailure >= int32(retry) {
			return nil, cErr.BadRequest("Please try again after 1 day")
		}
	}

	if err := password.Compare(user.Password, plainPassword); err != nil {
		s.users.UpdateLoginMeta(user.ID, user.LoginFailure+1, time.Now().Unix(), ip)
		return nil, cErr.BadRequest("Password is incorrect")
	}

	if s.config.App.UserSso {
		s.tokenHelper.Clear("user", user.ID)
		s.tokenHelper.Clear("user-refresh", user.ID)
	}
	refreshToken := ""
	if keep {
		refreshToken = random.Uuid()
		s.tokenHelper.Set(refreshToken, "user-refresh", user.ID, 2592000)
	}
	tokenStr := random.Uuid()
	if err := s.tokenHelper.Set(tokenStr, "user", user.ID, s.config.App.UserTokenKeepTime); err != nil {
		return nil, err
	}

	loginTime := time.Now().Unix()
	err = s.users.UpdateLoginMeta(user.ID, 0, loginTime, ip)
	user.LoginFailure = 0
	user.LastLoginTime = loginTime
	user.LastLoginIP = ip

	userInfo := s.FilterData(*user)
	userInfo["token"] = tokenStr
	userInfo["refresh_token"] = refreshToken
	return userInfo, err
}

func (s *MemberService) FilterData(user model.User) map[string]any {
	return map[string]any{
		"id":              user.ID,
		"username":        user.Username,
		"nickname":        user.Nickname,
		"email":           user.Email,
		"mobile":          user.Mobile,
		"avatar":          user.Avatar,
		"money":           fmt.Sprintf("%.2f", user.Money),
		"join_time":       user.JoinTime,
		"last_login_time": user.LastLoginTime,
		"last_login_ip":   user.LastLoginIP,
	}
}

// UserInfoByToken resolves the member behind an access token and returns the
// filtered user payload; ok is false when the token is missing/invalid or the
// member no longer exists. Used by the public api index endpoint to include
// userInfo only for logged-in requests (php auth->getUserInfo()).
func (s *MemberService) UserInfoByToken(tokenStr string) (map[string]any, bool) {
	if tokenStr == "" {
		return nil, false
	}
	tokenData, ok := s.IsLoginToken(tokenStr)
	if !ok {
		return nil, false
	}
	user, err := s.users.GetByID(tokenData.UserID)
	if err != nil {
		return nil, false
	}
	return s.FilterData(*user), true
}

// Register creates a member account and issues the access token. ip is passed
// explicitly so the flow stays transport-free.
func (s *MemberService) Register(ip string, username string, plainPassword string) (interface{}, error) {
	exists, err := s.accountExists("username", username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, cErr.BadRequest("Username is exist!")
	}

	hash, err := password.Hash(plainPassword)
	if err != nil {
		return nil, err
	}
	rootID, err := (systemroot.Resolver{
		DB:         s.users.DB(),
		AdminTable: s.config.Database.Prefix + "admin",
	}).Resolve()
	if err != nil {
		return nil, cErr.BadRequest("system root administrator is not configured")
	}

	now := time.Now().Unix()
	user := model.User{
		AdminID:       rootID,
		Username:      username,
		Nickname:      util.MaskPhone(username),
		Avatar:        "",
		Password:      hash,
		Status:        "enable",
		LastLoginTime: now,
		LastLoginIP:   ip,
		JoinIP:        ip,
		JoinTime:      now,
	}
	if err := s.users.Create(&user); err != nil {
		return nil, err
	}

	tokenStr := random.Uuid()
	if err := s.tokenHelper.Set(tokenStr, "user", user.ID, s.config.App.UserTokenKeepTime); err != nil {
		return nil, err
	}
	userInfo := s.FilterData(user)
	userInfo["token"] = tokenStr
	userInfo["refresh_token"] = ""
	return userInfo, nil
}

func (s *MemberService) accountExists(field, value string) (bool, error) {
	switch field {
	case "username", "email", "mobile":
	default:
		return false, cErr.BadRequest("invalid account field")
	}

	user, err := s.users.GetByAccount(field, value)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

// Logout invalidates the refresh token and the caller's access token.
func (s *MemberService) Logout(refreshToken string, accessToken string) error {
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
