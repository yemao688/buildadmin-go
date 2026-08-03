package service

import (
	"errors"
	"testing"

	adminauth "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/token"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// adminLoginRow mirrors the model.Admin column shape with sqlite-native
// types (the shared entity carries MySQL-dialect gorm tags that sqlite
// AutoMigrate cannot parse).
type adminLoginRow struct {
	ID            int32  `gorm:"column:id;primaryKey"`
	ParentID      *int32 `gorm:"column:parent_id"`
	Username      string `gorm:"column:username"`
	Nickname      string `gorm:"column:nickname"`
	Avatar        string `gorm:"column:avatar"`
	Email         string `gorm:"column:email"`
	Mobile        string `gorm:"column:mobile"`
	LoginFailure  int32  `gorm:"column:login_failure"`
	LastLoginTime int64  `gorm:"column:last_login_time"`
	LastLoginIP   string `gorm:"column:last_login_ip"`
	Password      string `gorm:"column:password"`
	Motto         string `gorm:"column:motto"`
	Status        string `gorm:"column:status"`
	UpdateTime    int64  `gorm:"column:update_time"`
	CreateTime    int64  `gorm:"column:create_time"`
}

func (adminLoginRow) TableName() string { return "admins" }

type authServiceTokenDriver struct{}

func (authServiceTokenDriver) Set(string, string, int32, int64) error { return nil }
func (authServiceTokenDriver) Get(string) (*token.Token, error)       { return nil, errors.New("not found") }
func (authServiceTokenDriver) Check(string, string, int32) bool       { return false }
func (authServiceTokenDriver) Delete(string) error                    { return nil }
func (authServiceTokenDriver) Clear(string, int32) error              { return nil }

func newAuthServiceFixture(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:admin-auth-service-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&adminLoginRow{}))
	config := &conf.Configuration{}
	config.App.AdminTokenKeepTime = 3600
	authRepo := adminauth.NewAuthRepository(db, &token.TokenHelper{Driver: authServiceTokenDriver{}}, config)
	return NewAuthService(config, authRepo, nil), db
}

func createLoginAdmin(t *testing.T, db *gorm.DB, username, plain, status string) adminLoginRow {
	t.Helper()
	hash, err := password.Hash(plain)
	require.NoError(t, err)
	row := adminLoginRow{Username: username, Nickname: username, Password: hash, Status: status}
	require.NoError(t, db.Create(&row).Error)
	return row
}

func TestAuthServiceLoginSuccessIssuesSession(t *testing.T) {
	svc, db := newAuthServiceFixture(t)
	createLoginAdmin(t, db, "root", "correct horse battery staple", "enable")

	result, err := svc.Login("root", "correct horse battery staple", false, "", "", "203.0.113.9")
	require.NoError(t, err)
	require.Equal(t, int32(1), result["id"])
	require.Equal(t, "root", result["username"])
	require.NotEmpty(t, result["token"])
	require.Equal(t, "", result["refresh_token"])

	var updated adminLoginRow
	require.NoError(t, db.First(&updated, 1).Error)
	require.Zero(t, updated.LoginFailure)
	require.Equal(t, "203.0.113.9", updated.LastLoginIP)
	require.NotZero(t, updated.LastLoginTime)
}

func TestAuthServiceLoginWrongPasswordRecordsFailure(t *testing.T) {
	svc, db := newAuthServiceFixture(t)
	createLoginAdmin(t, db, "root", "correct horse battery staple", "enable")

	_, err := svc.Login("root", "wrong", false, "", "", "203.0.113.9")
	require.EqualError(t, err, "Password is incorrect")

	var updated adminLoginRow
	require.NoError(t, db.First(&updated, 1).Error)
	require.Equal(t, int32(1), updated.LoginFailure)
}

func TestAuthServiceLoginDisabledAccountRejected(t *testing.T) {
	svc, db := newAuthServiceFixture(t)
	createLoginAdmin(t, db, "root", "correct horse battery staple", "disable")

	_, err := svc.Login("root", "correct horse battery staple", false, "", "", "203.0.113.9")
	require.EqualError(t, err, "Username is incorrect")
}

func TestAuthServiceLoginUnknownAccountRejected(t *testing.T) {
	svc, _ := newAuthServiceFixture(t)
	_, err := svc.Login("ghost", "whatever", false, "", "", "203.0.113.9")
	require.EqualError(t, err, "Username is incorrect")
}

func TestAuthServiceLoginCaptchaRequired(t *testing.T) {
	svc, db := newAuthServiceFixture(t)
	svc.config.App.AdminLoginCaptcha = true
	createLoginAdmin(t, db, "root", "correct horse battery staple", "enable")

	// Missing captcha payload is rejected before any credential check.
	_, err := svc.Login("root", "correct horse battery staple", false, "", "", "203.0.113.9")
	require.EqualError(t, err, "Captcha error")

	// A non-empty payload still fails when no valid captcha record exists.
	_, err = svc.Login("root", "correct horse battery staple", false, "cap-1", "1,2;3,4", "203.0.113.9")
	require.EqualError(t, err, "Captcha error")
}

func TestAuthServiceLoginKeepIssuesRefreshToken(t *testing.T) {
	svc, db := newAuthServiceFixture(t)
	createLoginAdmin(t, db, "root", "correct horse battery staple", "enable")

	result, err := svc.Login("root", "correct horse battery staple", true, "", "", "203.0.113.9")
	require.NoError(t, err)
	require.NotEmpty(t, result["refresh_token"])
}

func TestAuthServiceLoginRetryLockout(t *testing.T) {
	svc, db := newAuthServiceFixture(t)
	svc.config.App.AdminLoginRetry = 2
	createLoginAdmin(t, db, "root", "correct horse battery staple", "enable")

	_, err := svc.Login("root", "wrong", false, "", "", "1.1.1.1")
	require.EqualError(t, err, "Password is incorrect")
	_, err = svc.Login("root", "wrong", false, "", "", "1.1.1.1")
	require.EqualError(t, err, "Password is incorrect")
	// Third attempt within the same day is locked out before password check.
	_, err = svc.Login("root", "correct horse battery staple", false, "", "", "1.1.1.1")
	require.EqualError(t, err, "Please try again after 1 day")
}

func TestAuthServiceIsLoggedIn(t *testing.T) {
	svc, _ := newAuthServiceFixture(t)
	require.False(t, svc.IsLoggedIn(""))
	require.False(t, svc.IsLoggedIn("some-token"))
}
