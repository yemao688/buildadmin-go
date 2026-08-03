package member

import (
	"fmt"
	"buildadmin-go/internal/api/repository/user"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	cErr "buildadmin-go/internal/pkg/error"
	"buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/random"
	"buildadmin-go/internal/pkg/systemroot"
	"buildadmin-go/internal/pkg/token"
	"regexp"
	"time"

	"gorm.io/gorm"

	"buildadmin-go/internal/utils"
)

type Service struct {
	users       *repository.Repository
	tokenHelper *token.TokenHelper
	config      *conf.Configuration
}

var (
	loginPhoneRegex    = regexp.MustCompile(`^1[3-9]\d{9}$`)
	loginEmailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	loginUsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{4,30}$`)
)

func NewService(sqlDB *gorm.DB, tokenHelper *token.TokenHelper, config *conf.Configuration) *Service {
	return &Service{users: repository.NewRepository(sqlDB), tokenHelper: tokenHelper, config: config}
}

// IsLoginToken resolves a user session token. The header extraction from the
// request is a transport concern handled by the caller.
func (s *Service) IsLoginToken(tokenStr string) (*token.Token, bool) {
	if tokenStr == "" {
		return nil, false
	}
	tokenData, err := s.tokenHelper.GetFor(tokenStr, "user")
	if err == nil {
		return tokenData, true
	}
	return nil, false
}

func (s *Service) IsEnabledUser(id int32) bool {
	return s.users.IsEnabled(id)
}

func (s *Service) RefreshUserAccessToken(refreshToken string) (string, error) {
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
		if !utils.AccountStatusEnabled(user.Status) {
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

func (s *Service) ValidateUserToken(id int32, ip string) error {
	user, err := s.users.GetByID(id)
	if err != nil {
		return err
	}
	if user == nil {
		return cErr.BadRequest("Account not exist")
	}
	if user.Status != "enable" {
		return cErr.BadRequest("Account disabled")
	}
	return s.users.UpdateLoginMeta(id, 0, time.Now().Unix(), ip)
}

// Login verifies the credentials and issues the member tokens. ip is passed
// explicitly so the flow stays transport-free.
func (s *Service) Login(ip string, username string, plainPassword string, keep bool) (interface{}, error) {
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
	if !utils.AccountStatusEnabled(user.Status) {
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

func (s *Service) FilterData(user model.User) map[string]any {
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

// Register creates a member account and issues the access token. ip is passed
// explicitly so the flow stays transport-free.
func (s *Service) Register(ip string, username string, plainPassword string) (interface{}, error) {
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
		Nickname:      utils.MaskPhone(username),
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

func (s *Service) accountExists(field, value string) (bool, error) {
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
func (s *Service) Logout(refreshToken string, accessToken string) error {
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
