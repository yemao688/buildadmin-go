package model

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/token"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type authTestTokenDriver struct{}

func (authTestTokenDriver) Set(string, string, int32, int64) error { return nil }
func (authTestTokenDriver) Get(string) (*token.Token, error)       { return nil, errors.New("not found") }
func (authTestTokenDriver) Check(string, string, int32) bool       { return false }
func (authTestTokenDriver) Delete(string) error                    { return nil }
func (authTestTokenDriver) Clear(string, int32) error              { return nil }

func newAuthTestModel(t *testing.T) (*AuthModel, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:auth-model?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	config := &conf.Configuration{}
	config.Database.Prefix = ""
	config.App.UserTokenKeepTime = 3600
	return NewAuthModel(db, &token.TokenHelper{Driver: authTestTokenDriver{}}, config), db
}

func authTestContext() *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/", nil)
	return ctx
}

func TestAuthLoginMissingAccountIsNotDisabled(t *testing.T) {
	m, _ := newAuthTestModel(t)
	_, err := m.Login(authTestContext(), "user_404", "password", false)
	require.EqualError(t, err, "Account not exist")
}

func TestAuthLoginAllowsLegacyStatuses(t *testing.T) {
	m, db := newAuthTestModel(t)
	for _, status := range []string{"0", "1", "other"} {
		salt := "salt-" + status
		user := User{Username: "user_" + status, Password: utils.EncryptPassword("password", salt), Salt: salt, Status: status}
		require.NoError(t, db.Create(&user).Error)
		_, err := m.Login(authTestContext(), user.Username, "password", false)
		require.NoError(t, err)
	}
	user := User{Username: "disabled", Password: utils.EncryptPassword("password", "disabled-salt"), Salt: "disabled-salt", Status: "disable"}
	require.NoError(t, db.Create(&user).Error)
	_, err := m.Login(authTestContext(), user.Username, "password", false)
	require.EqualError(t, err, "Account disabled")
}

func TestAuthLoginUsesMobileBeforeUsername(t *testing.T) {
	m, db := newAuthTestModel(t)
	salt := "mobile-salt"
	user := User{
		Username: "phone_user",
		Mobile:   "18888888888",
		Password: utils.EncryptPassword("password", salt),
		Salt:     salt,
		Status:   "enable",
	}
	require.NoError(t, db.Create(&user).Error)

	_, err := m.Login(authTestContext(), user.Mobile, "password", false)
	require.NoError(t, err)
}

func TestAuthLoginResetsExpiredFailureCounter(t *testing.T) {
	m, db := newAuthTestModel(t)
	m.config.App.UserLoginRetry = 2
	user := User{
		Username:      "cooldown_user",
		Password:      utils.EncryptPassword("correct", "cooldown-salt"),
		Salt:          "cooldown-salt",
		Status:        "enable",
		LoginFailure:  2,
		LastLoginTime: time.Now().Unix() - 86400,
	}
	require.NoError(t, db.Create(&user).Error)

	_, err := m.Login(authTestContext(), user.Username, "wrong", false)
	require.EqualError(t, err, "Password is incorrect")
	var updated User
	require.NoError(t, db.First(&updated, user.ID).Error)
	require.Equal(t, int32(1), updated.LoginFailure)
}

func TestAuthLoginReturnsUpdatedLastLoginFields(t *testing.T) {
	m, db := newAuthTestModel(t)
	salt := "response-salt"
	user := User{
		Username: "response_user",
		Password: utils.EncryptPassword("password", salt),
		Salt:     salt,
		Status:   "enable",
	}
	require.NoError(t, db.Create(&user).Error)
	ctx := authTestContext()
	ctx.Request.RemoteAddr = "203.0.113.9:1234"

	result, err := m.Login(ctx, user.Username, "password", false)
	require.NoError(t, err)
	data := result.(map[string]any)
	require.Equal(t, "203.0.113.9", data["last_login_ip"])
	require.NotZero(t, data["last_login_time"])
}

func TestAuthRegisterRejectsExistingContactFields(t *testing.T) {
	for _, test := range []struct {
		name, username, email, mobile, message string
	}{
		{"username", "existing", "", "", "Username is exist!"},
		{"email", "", "existing@example.com", "", "Email is exist!"},
		{"mobile", "", "", "13800000000", "Mobile is exist!"},
	} {
		t.Run(test.name, func(t *testing.T) {
			m, db := newAuthTestModel(t)
			require.NoError(t, db.Create(&User{Username: test.username, Email: test.email, Mobile: test.mobile}).Error)
			_, err := m.Register(authTestContext(), test.username, "password", test.mobile, test.email)
			require.EqualError(t, err, test.message)
		})
	}
}

func TestAuthRegisterReturnsDatabaseErrorDuringUniquenessCheck(t *testing.T) {
	m, db := newAuthTestModel(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = m.Register(authTestContext(), "new-user", "password", "", "")
	require.Error(t, err)
	require.NotEqual(t, "Username is exist!", err.Error())
}
