package service

import (
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/clickcaptcha"
	cErr "buildadmin-go/internal/pkg/error"
)

// AuthService 承载后台登录/登出流：验证码校验、凭据验证与 token 签发。
// 登录日志由 Record 中间件统一落库，不在此重复。
type AuthService struct {
	config       *conf.Configuration
	authM        *adminmodel.AuthRepository
	clickCaptcha *clickcaptcha.ClickCaptcha
}

func NewAuthService(config *conf.Configuration, authM *adminmodel.AuthRepository, clickCaptcha *clickcaptcha.ClickCaptcha) *AuthService {
	return &AuthService{config: config, authM: authM, clickCaptcha: clickCaptcha}
}

// IsLoggedIn reports whether a batoken still resolves to an admin session.
// The token extraction from the request is a transport concern handled by
// the caller.
func (s *AuthService) IsLoggedIn(batoken string) bool {
	if batoken == "" {
		return false
	}
	_, ok := s.authM.TokenInfo(batoken)
	return ok
}

// Login verifies the captcha (when enabled) and the credentials, then issues
// the admin access/refresh tokens. clientIP is passed explicitly so the flow
// stays transport-free.
func (s *AuthService) Login(username, password string, keep bool, captchaID, captchaInfo, clientIP string) (map[string]any, error) {
	if s.config.App.AdminLoginCaptcha {
		if captchaID == "" || captchaInfo == "" {
			return nil, cErr.BadRequest("Captcha error")
		}
		if s.clickCaptcha == nil || !s.clickCaptcha.Check(captchaID, captchaInfo, true) {
			return nil, cErr.BadRequest("Captcha error")
		}
	}

	result, err := s.authM.Login(username, password, keep, clientIP)
	if err != nil {
		return nil, err
	}
	loginResult, ok := result.(map[string]interface{})
	if !ok {
		return nil, cErr.InternalServer("Invalid login result")
	}
	adminID, ok := loginResult["id"].(int32)
	if !ok || adminID == 0 {
		return nil, cErr.InternalServer("Invalid login result")
	}
	return loginResult, nil
}

// Logout invalidates the refresh token and the caller's access token.
func (s *AuthService) Logout(refreshToken, accessToken string) error {
	return s.authM.Logout(refreshToken, accessToken)
}
