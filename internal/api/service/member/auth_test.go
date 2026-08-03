package member

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/pkg/token"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type authTestTokenDriver struct{}

func (authTestTokenDriver) Set(string, string, int32, int64) error { return nil }
func (authTestTokenDriver) Get(string) (*token.Token, error)       { return nil, errors.New("not found") }
func (authTestTokenDriver) Check(string, string, int32) bool       { return false }
func (authTestTokenDriver) Delete(string) error                    { return nil }
func (authTestTokenDriver) Clear(string, int32) error              { return nil }

type authDomainTokenDriver struct {
	data *token.Token
}

func (d authDomainTokenDriver) Set(string, string, int32, int64) error { return nil }
func (d authDomainTokenDriver) Get(string) (*token.Token, error)       { return d.data, nil }
func (d authDomainTokenDriver) Check(string, string, int32) bool       { return false }
func (d authDomainTokenDriver) Delete(string) error                    { return nil }
func (d authDomainTokenDriver) Clear(string, int32) error              { return nil }

func newAuthTestModel(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:auth-model?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteUserTables(db, "users", "admins"))
	config := &conf.Configuration{}
	config.Database.Prefix = ""
	config.App.UserTokenKeepTime = 3600
	return NewService(db, &token.TokenHelper{Driver: authTestTokenDriver{}}, config), db
}

func authTestContext() *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/", nil)
	return ctx
}

func hashForTest(t *testing.T, plain string) string {
	t.Helper()
	hash, err := password.Hash(plain)
	require.NoError(t, err)
	return hash
}

func TestAuthIsLoginRejectsAdminToken(t *testing.T) {
	m, _ := newAuthTestModel(t)
	m.tokenHelper = &token.TokenHelper{Driver: authDomainTokenDriver{data: &token.Token{Type: "admin", UserID: 1}}}

	got, ok := m.IsLoginToken("admin-token")
	require.False(t, ok)
	require.Nil(t, got)
}

func TestAuthLoginMissingAccountIsNotDisabled(t *testing.T) {
	m, _ := newAuthTestModel(t)
	_, err := m.Login("203.0.113.9", "user_404", "password", false)
	require.EqualError(t, err, "Account not exist")
}

func TestAuthLoginUsesBcryptAndStrictStatuses(t *testing.T) {
	m, db := newAuthTestModel(t)
	for _, status := range []string{"enable", "disable"} {
		user := model.User{Username: "user_" + status, Password: hashForTest(t, "password"), Status: status}
		require.NoError(t, db.Create(&user).Error)
		_, err := m.Login("203.0.113.9", user.Username, "password", false)
		if status == "enable" {
			require.NoError(t, err)
		} else {
			require.EqualError(t, err, "Account disabled")
		}
	}
}

func TestAuthLoginResetsExpiredFailureCounter(t *testing.T) {
	m, db := newAuthTestModel(t)
	m.config.App.UserLoginRetry = 2
	user := model.User{
		Username:      "cooldown_user",
		Password:      hashForTest(t, "correct"),
		Status:        "enable",
		LoginFailure:  2,
		LastLoginTime: time.Now().Unix() - 86400,
	}
	require.NoError(t, db.Create(&user).Error)

	_, err := m.Login("203.0.113.9", user.Username, "wrong", false)
	require.EqualError(t, err, "Password is incorrect")
	var updated model.User
	require.NoError(t, db.First(&updated, user.ID).Error)
	require.Equal(t, int32(1), updated.LoginFailure)
}

func TestAuthLoginReturnsUpdatedLastLoginFields(t *testing.T) {
	m, db := newAuthTestModel(t)
	user := model.User{Username: "response_user", Password: hashForTest(t, "password"), Status: "enable"}
	require.NoError(t, db.Create(&user).Error)

	result, err := m.Login("203.0.113.9", user.Username, "password", false)
	require.NoError(t, err)
	data := result.(map[string]any)
	require.Equal(t, "203.0.113.9", data["last_login_ip"])
	require.NotZero(t, data["last_login_time"])
}

func TestAuthRegisterRejectsExistingUsername(t *testing.T) {
	m, db := newAuthTestModel(t)
	require.NoError(t, db.Create(&model.User{Username: "existing"}).Error)
	_, err := m.Register("203.0.113.9", "existing", "password")
	require.EqualError(t, err, "Username is exist!")
}

func TestAuthRegisterReturnsDatabaseErrorDuringUniquenessCheck(t *testing.T) {
	m, db := newAuthTestModel(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = m.Register("203.0.113.9", "new-user", "password")
	require.Error(t, err)
	require.NotEqual(t, "Username is exist!", err.Error())
}
