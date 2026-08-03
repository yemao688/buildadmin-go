package service

import (
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/clickcaptcha"
	cErr "buildadmin-go/internal/pkg/error"
	passwordutil "buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/random"
	"buildadmin-go/internal/pkg/token"
	"buildadmin-go/internal/utils"
	"time"
)

// adminLoginCredentialError is the single generic message for every failed
// credential check (unknown account, disabled account, wrong password) so
// the login endpoint cannot be used to enumerate accounts.
const adminLoginCredentialError = "Incorrect user name or password!"

// AuthService 承载后台登录/登出流：验证码校验、凭据验证、节流决策、token
// 签发与失败补偿。数据访问（管理员读取、登录态行锁、失败计数）经
// AuthRepository 原语；登录日志由 Record 中间件统一落库，不在此重复。
type AuthService struct {
	config       *conf.Configuration
	authM        *adminmodel.AuthRepository
	clickCaptcha *clickcaptcha.ClickCaptcha
	tokenHelper  *token.TokenHelper
}

func NewAuthService(config *conf.Configuration, authM *adminmodel.AuthRepository, clickCaptcha *clickcaptcha.ClickCaptcha, tokenHelper *token.TokenHelper) *AuthService {
	return &AuthService{config: config, authM: authM, clickCaptcha: clickCaptcha, tokenHelper: tokenHelper}
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
// stays transport-free. Every failure path either leaves no token behind or
// compensates the already-written ones.
func (s *AuthService) Login(username, password string, keep bool, captchaID, captchaInfo, clientIP string) (map[string]any, error) {
	if s.config.App.AdminLoginCaptcha {
		if captchaID == "" || captchaInfo == "" {
			return nil, cErr.BadRequest("Captcha error")
		}
		if s.clickCaptcha == nil || !s.clickCaptcha.Check(captchaID, captchaInfo, true) {
			return nil, cErr.BadRequest("Captcha error")
		}
	}

	admin, err := s.authM.GetByUsername(username)
	if err != nil {
		return nil, err
	}
	if admin.ID == 0 || !utils.AccountStatusEnabled(admin.Status) {
		// Unknown and disabled accounts share the generic credential error.
		return nil, cErr.BadRequest(adminLoginCredentialError)
	}

	now := time.Now().Unix()
	locked, err := s.authM.LockLoginState(admin.ID, now, s.config.App.AdminLoginRetry)
	if err != nil {
		return nil, err
	}
	if locked {
		return nil, cErr.BadRequest("Please try again after 1 day")
	}

	if err := passwordutil.Compare(admin.Password, password); err != nil {
		// Atomic increment: concurrent failures cannot overwrite each other
		// and the counter stays monotonic. The error propagates instead of
		// being swallowed.
		if updErr := s.authM.IncrementLoginFailure(admin.ID, now, clientIP); updErr != nil {
			return nil, updErr
		}
		return nil, cErr.BadRequest(adminLoginCredentialError)
	}

	// SSO: previous sessions are cleared before a new one is issued; a failed
	// clear aborts the login instead of leaving stale sessions behind.
	if s.config.App.AdminSso {
		if err := s.tokenHelper.Clear("admin", admin.ID); err != nil {
			return nil, err
		}
		if err := s.tokenHelper.Clear("admin-refresh", admin.ID); err != nil {
			return nil, err
		}
	}

	refreshToken := ""
	if keep {
		refreshToken = random.Uuid()
		if err := s.tokenHelper.Set(refreshToken, "admin-refresh", admin.ID, 2592000); err != nil { //30天
			return nil, err
		}
	}
	token := random.Uuid()
	if err := s.tokenHelper.Set(token, "admin", admin.ID, s.config.App.AdminTokenKeepTime); err != nil {
		// Compensate: the refresh token was already written and must not
		// outlive the failed login.
		if refreshToken != "" {
			_ = s.tokenHelper.Delete(refreshToken)
		}
		return nil, err
	}

	// The login metadata update must succeed; on failure every issued token
	// is revoked so an error response never leaves a usable session behind.
	if err := s.authM.ResetLoginMeta(admin.ID, now, clientIP); err != nil {
		_ = s.tokenHelper.Delete(token)
		if refreshToken != "" {
			_ = s.tokenHelper.Delete(refreshToken)
		}
		return nil, err
	}

	return map[string]any{
		"id":              admin.ID,
		"username":        admin.Username,
		"nickname":        admin.Nickname,
		"avatar":          admin.Avatar,
		"last_login_time": now,
		"token":           token,
		"refresh_token":   refreshToken,
	}, nil
}

// Logout invalidates the refresh token and the caller's access token.
func (s *AuthService) Logout(refreshToken, accessToken string) error {
	return s.authM.Logout(refreshToken, accessToken)
}
