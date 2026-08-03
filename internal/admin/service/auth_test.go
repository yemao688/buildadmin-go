package service

import (
	"errors"
	"testing"
	"time"

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
	require.EqualError(t, err, "Incorrect user name or password!")

	var updated adminLoginRow
	require.NoError(t, db.First(&updated, 1).Error)
	require.Equal(t, int32(1), updated.LoginFailure)
}

func TestAuthServiceLoginDisabledAccountRejected(t *testing.T) {
	svc, db := newAuthServiceFixture(t)
	createLoginAdmin(t, db, "root", "correct horse battery staple", "disable")

	_, err := svc.Login("root", "correct horse battery staple", false, "", "", "203.0.113.9")
	require.EqualError(t, err, "Incorrect user name or password!")
}

func TestAuthServiceLoginUnknownAccountRejected(t *testing.T) {
	svc, _ := newAuthServiceFixture(t)
	_, err := svc.Login("ghost", "whatever", false, "", "", "203.0.113.9")
	require.EqualError(t, err, "Incorrect user name or password!")
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
	require.EqualError(t, err, "Incorrect user name or password!")
	_, err = svc.Login("root", "wrong", false, "", "", "1.1.1.1")
	require.EqualError(t, err, "Incorrect user name or password!")
	// Third attempt within the same day is locked out before password check.
	_, err = svc.Login("root", "correct horse battery staple", false, "", "", "1.1.1.1")
	require.EqualError(t, err, "Please try again after 1 day")
}

func TestAuthServiceLoginExpiredWindowResetsCounter(t *testing.T) {
	svc, db := newAuthServiceFixture(t)
	svc.config.App.AdminLoginRetry = 2
	hash, err := password.Hash("correct horse battery staple")
	require.NoError(t, err)
	// Two stale failures whose lockout window expired a minute ago.
	row := adminLoginRow{
		Username:      "cooled",
		Nickname:      "cooled",
		Password:      hash,
		Status:        "enable",
		LoginFailure:  2,
		LastLoginTime: time.Now().Unix() - 86400 - 60,
	}
	require.NoError(t, db.Create(&row).Error)

	// One wrong attempt after the expired window: the stale counter is
	// cleared first, so the failure count restarts at 1 instead of 3.
	_, err = svc.Login("cooled", "wrong", false, "", "", "203.0.113.9")
	require.EqualError(t, err, "Incorrect user name or password!")

	var updated adminLoginRow
	require.NoError(t, db.First(&updated, row.ID).Error)
	require.Equal(t, int32(1), updated.LoginFailure)

	// With the counter restarted the account is not locked: the correct
	// password now succeeds.
	result, err := svc.Login("cooled", "correct horse battery staple", false, "", "", "203.0.113.9")
	require.NoError(t, err)
	require.Equal(t, row.ID, result["id"])
}

func TestAuthServiceLoginCounterMonotonic(t *testing.T) {
	svc, db := newAuthServiceFixture(t)
	// Throttling disabled: every wrong attempt must still be counted and
	// the counter must never regress.
	createLoginAdmin(t, db, "root", "correct horse battery staple", "enable")

	for i := 0; i < 3; i++ {
		_, err := svc.Login("root", "wrong", false, "", "", "203.0.113.9")
		require.EqualError(t, err, "Incorrect user name or password!")
	}
	var updated adminLoginRow
	require.NoError(t, db.First(&updated, 1).Error)
	require.Equal(t, int32(3), updated.LoginFailure)
}

func TestAuthServiceIsLoggedIn(t *testing.T) {
	svc, _ := newAuthServiceFixture(t)
	require.False(t, svc.IsLoggedIn(""))
	require.False(t, svc.IsLoggedIn("some-token"))
}

// failingAccessTokenDriver records deletions and fails on the access-token
// Set, simulating a token store outage after the refresh token was written.
type failingAccessTokenDriver struct {
	deleted    []string
	refreshSet bool
}

func (d *failingAccessTokenDriver) Set(tokenStr, typ string, userID int32, expire int64) error {
	if typ == "admin-refresh" {
		d.refreshSet = true
		return nil
	}
	if typ == "admin" && d.refreshSet {
		return errors.New("access token store unavailable")
	}
	return nil
}
func (d *failingAccessTokenDriver) Get(string) (*token.Token, error) {
	return nil, errors.New("not found")
}
func (d *failingAccessTokenDriver) Check(string, string, int32) bool { return false }
func (d *failingAccessTokenDriver) Delete(tokenStr string) error {
	d.deleted = append(d.deleted, tokenStr)
	return nil
}
func (d *failingAccessTokenDriver) Clear(string, int32) error { return nil }

func TestAuthServiceLoginCompensatesAccessTokenSetFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-auth-compensate-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&adminLoginRow{}))
	driver := &failingAccessTokenDriver{}
	config := &conf.Configuration{}
	config.App.AdminTokenKeepTime = 3600
	authRepo := adminauth.NewAuthRepository(db, &token.TokenHelper{Driver: driver}, config)
	svc := NewAuthService(config, authRepo, nil)
	createLoginAdmin(t, db, "root", "correct horse battery staple", "enable")

	_, err = svc.Login("root", "correct horse battery staple", true, "", "", "203.0.113.9")
	require.Error(t, err)
	// The refresh token written before the access-token failure must be
	// compensated (deleted) so no residual session outlives the error.
	require.Len(t, driver.deleted, 1)
	require.NotEmpty(t, driver.deleted[0])
}

// ssoFailDriver errors on the SSO clear of previous sessions.
type ssoFailDriver struct{}

func (ssoFailDriver) Set(string, string, int32, int64) error { return nil }
func (ssoFailDriver) Get(string) (*token.Token, error)       { return nil, errors.New("not found") }
func (ssoFailDriver) Check(string, string, int32) bool       { return false }
func (ssoFailDriver) Delete(string) error                    { return nil }
func (ssoFailDriver) Clear(string, int32) error              { return errors.New("clear unavailable") }

func TestAuthServiceLoginPropagatesSSOClearFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-auth-ssofail-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&adminLoginRow{}))
	config := &conf.Configuration{}
	config.App.AdminTokenKeepTime = 3600
	config.App.AdminSso = true
	authRepo := adminauth.NewAuthRepository(db, &token.TokenHelper{Driver: ssoFailDriver{}}, config)
	svc := NewAuthService(config, authRepo, nil)
	createLoginAdmin(t, db, "root", "correct horse battery staple", "enable")

	_, err = svc.Login("root", "correct horse battery staple", false, "", "", "203.0.113.9")
	require.EqualError(t, err, "clear unavailable")
}
