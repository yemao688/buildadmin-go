package member

import (
	"errors"
	"fmt"
	"go-build-admin/app/common/model"
	cErr "go-build-admin/app/pkg/error"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/password"
	"go-build-admin/app/pkg/random"
	"go-build-admin/app/pkg/systemroot"
	"go-build-admin/app/pkg/token"
	"go-build-admin/conf"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"go-build-admin/utils"
)

type Service struct {
	sqlDB       *gorm.DB
	tokenHelper *token.TokenHelper
	config      *conf.Configuration
}

var (
	loginPhoneRegex    = regexp.MustCompile(`^1[3-9]\d{9}$`)
	loginEmailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	loginUsernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{4,30}$`)
)

func NewService(sqlDB *gorm.DB, tokenHelper *token.TokenHelper, config *conf.Configuration) *Service {
	return &Service{sqlDB: sqlDB, tokenHelper: tokenHelper, config: config}
}

func (s *Service) IsLogin(ctx *gin.Context) (*token.Token, bool) {
	tokenStr := ctx.Request.Header.Get("ba-user-token")
	if tokenStr != "" {
		tokenData, err := s.tokenHelper.GetFor(tokenStr, "user")
		if err == nil {
			return tokenData, true
		}
	}
	return nil, false
}

func (s *Service) IsEnabledUser(id int32) bool {
	var user model.User
	err := s.sqlDB.Model(&model.User{}).Select("status").Where("id=?", id).First(&user).Error
	return err == nil && user.Status == "enable"
}

func (s *Service) RefreshUserAccessToken(ctx *gin.Context, refreshToken string) (string, error) {
	initial, err := s.tokenHelper.Get(refreshToken)
	if err != nil {
		return "", err
	}
	if initial.Type != "user-refresh" {
		return "", cErr.BadRequest("Invalid token")
	}

	newToken := ""
	err = s.sqlDB.Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", initial.UserID).First(&user).Error; err != nil {
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

func (s *Service) ValidateUserToken(ctx *gin.Context, id int32, ip string) error {
	var user model.User
	if err := s.sqlDB.Where("id=?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cErr.BadRequest("Account not exist")
		}
		return err
	}
	if user.Status != "enable" {
		return cErr.BadRequest("Account disabled")
	}
	return s.sqlDB.Model(&model.User{}).Where("id=?", id).Updates(map[string]any{
		"login_failure":   0,
		"last_login_time": time.Now().Unix(),
		"last_login_ip":   ip,
	}).Error
}

func (s *Service) Login(ctx *gin.Context, username string, plainPassword string, keep bool) (interface{}, error) {
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

	user := model.User{}
	result := s.sqlDB.Model(&model.User{}).Where(accountType+"=?", username).Scan(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, cErr.BadRequest("Account not exist")
	}
	if !utils.AccountStatusEnabled(user.Status) {
		return nil, cErr.BadRequest("Account disabled")
	}

	retry := s.config.App.UserLoginRetry
	if retry > 0 && user.LastLoginTime > 0 {
		now := time.Now().Unix()
		if user.LoginFailure > 0 && now-user.LastLoginTime >= 86400 {
			if err := s.sqlDB.Model(&model.User{}).Where("id=?", user.ID).Updates(map[string]any{"login_failure": 0}).Error; err != nil {
				return nil, err
			}
			result = s.sqlDB.Model(&model.User{}).Where(accountType+"=?", username).Scan(&user)
			if result.Error != nil {
				return nil, result.Error
			}
		}
		if user.LoginFailure >= int32(retry) {
			return nil, cErr.BadRequest("Please try again after 1 day")
		}
	}

	if err := password.Compare(user.Password, plainPassword); err != nil {
		s.sqlDB.Model(&model.User{}).Where("id=?", user.ID).Updates(map[string]interface{}{
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
		s.tokenHelper.Set(refreshToken, "user-refresh", user.ID, 2592000)
	}
	tokenStr := random.Uuid()
	if err := s.tokenHelper.Set(tokenStr, "user", user.ID, s.config.App.UserTokenKeepTime); err != nil {
		return nil, err
	}

	loginTime := time.Now().Unix()
	loginIP := ctx.ClientIP()
	err := s.sqlDB.Model(&model.User{}).Where("id=?", user.ID).Updates(map[string]interface{}{
		"login_failure":   0,
		"last_login_time": loginTime,
		"last_login_ip":   loginIP,
	}).Error
	user.LoginFailure = 0
	user.LastLoginTime = loginTime
	user.LastLoginIP = loginIP

	userInfo := s.FilterData(user)
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

func (s *Service) Register(ctx *gin.Context, username string, plainPassword string) (interface{}, error) {
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
		DB:         s.sqlDB,
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
		LastLoginIP:   ctx.ClientIP(),
		JoinIP:        ctx.ClientIP(),
		JoinTime:      now,
	}
	if err := s.sqlDB.Create(&user).Error; err != nil {
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

	var user model.User
	result := s.sqlDB.Model(&model.User{}).Where(field+"=?", value).Scan(&user)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (s *Service) Logout(ctx *gin.Context, refreshToken string) error {
	if refreshToken != "" {
		if err := s.tokenHelper.Delete(refreshToken); err != nil {
			return err
		}
	}
	userAuth := header.GetUserAuth(ctx)
	return s.tokenHelper.Delete(userAuth.Token)
}
