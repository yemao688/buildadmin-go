package service

import (
	"errors"
	"net/http/httptest"
	"sync"
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

func newAuthTestModel(t *testing.T) (*MemberService, *gorm.DB) {
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
	return NewMemberService(db, &token.TokenHelper{Driver: authTestTokenDriver{}}, config), db
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

func TestMemberIsLoginRejectsAdminToken(t *testing.T) {
	m, _ := newAuthTestModel(t)
	m.tokenHelper = &token.TokenHelper{Driver: authDomainTokenDriver{data: &token.Token{Type: "admin", UserID: 1}}}

	got, ok := m.IsLoginToken("admin-token")
	require.False(t, ok)
	require.Nil(t, got)
}

func TestMemberLoginMissingAccountIsNotDisabled(t *testing.T) {
	m, _ := newAuthTestModel(t)
	_, err := m.Login("203.0.113.9", "user_404", "password", false)
	require.EqualError(t, err, "Account not exist")
}

func TestMemberLoginUsesBcryptAndStrictStatuses(t *testing.T) {
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

func TestMemberLoginResetsExpiredFailureCounter(t *testing.T) {
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

func TestMemberLoginReturnsUpdatedLastLoginFields(t *testing.T) {
	m, db := newAuthTestModel(t)
	user := model.User{Username: "response_user", Password: hashForTest(t, "password"), Status: "enable"}
	require.NoError(t, db.Create(&user).Error)

	result, err := m.Login("203.0.113.9", user.Username, "password", false)
	require.NoError(t, err)
	data := result.(map[string]any)
	require.Equal(t, "203.0.113.9", data["last_login_ip"])
	require.NotZero(t, data["last_login_time"])
}

func TestMemberRegisterRejectsExistingUsername(t *testing.T) {
	m, db := newAuthTestModel(t)
	require.NoError(t, db.Create(&model.User{Username: "existing"}).Error)
	_, err := m.Register("203.0.113.9", "existing", "password")
	require.EqualError(t, err, "Username is exist!")
}

func TestMemberRegisterReturnsDatabaseErrorDuringUniquenessCheck(t *testing.T) {
	m, db := newAuthTestModel(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = m.Register("203.0.113.9", "new-user", "password")
	require.Error(t, err)
	require.NotEqual(t, "Username is exist!", err.Error())
}

// TestValidateUserTokenStatusSemantics 覆盖 H6a 合并后的行为保持：账号
// enable 通过、disable 与不存在均返回与旧 IsEnabledUser 分支一致的
// "Please login first"（enable/disable 双态，不涉及其它状态字段）。
func TestValidateUserTokenStatusSemantics(t *testing.T) {
	m, db := newAuthTestModel(t)
	for _, status := range []string{"enable", "disable"} {
		user := model.User{Username: "vut_" + status, Password: hashForTest(t, "password"), Status: status}
		require.NoError(t, db.Create(&user).Error)
		err := m.ValidateUserToken(user.ID, "203.0.113.9")
		if status == "enable" {
			require.NoError(t, err)
		} else {
			require.EqualError(t, err, "Please login first")
		}
	}
	require.EqualError(t, m.ValidateUserToken(99999, "203.0.113.9"), "Please login first")
}

// TestValidateUserTokenThrottlesLoginMetaWrites 覆盖 H6b 节流：同一 uid 在
// 60s 窗口内的第二次请求不触发写库（login 元信息保持第一次写入值），窗口
// 过后恢复写库。时钟经 loginMetaTTL.now 注入（测试入口，不导出公开 API）。
func TestValidateUserTokenThrottlesLoginMetaWrites(t *testing.T) {
	m, db := newAuthTestModel(t)
	user := model.User{Username: "throttle_user", Password: hashForTest(t, "password"), Status: "enable"}
	require.NoError(t, db.Create(&user).Error)

	now := time.Now()
	m.loginMetaTTL.now = func() time.Time { return now }

	// 第一次请求写库
	require.NoError(t, m.ValidateUserToken(user.ID, "198.51.100.7"))
	var first model.User
	require.NoError(t, db.First(&first, user.ID).Error)
	require.Equal(t, "198.51.100.7", first.LastLoginIP)

	// 同一 uid 在 60s 窗口内的第二次请求：不写库（IP 与时间均保持第一次值）
	now = now.Add(30 * time.Second)
	require.NoError(t, m.ValidateUserToken(user.ID, "198.51.100.9"))
	var second model.User
	require.NoError(t, db.First(&second, user.ID).Error)
	require.Equal(t, "198.51.100.7", second.LastLoginIP)
	require.Equal(t, first.LastLoginTime, second.LastLoginTime)

	// 窗口过后（已超过 60s）：恢复写库
	now = now.Add(31 * time.Second)
	require.NoError(t, m.ValidateUserToken(user.ID, "198.51.100.11"))
	var third model.User
	require.NoError(t, db.First(&third, user.ID).Error)
	require.Equal(t, "198.51.100.11", third.LastLoginIP)
}

// TestLoginMetaThrottleConcurrentSafe 验证节流器并发安全（配合 go test -race
// 检查 map 读写均持锁）。
func TestLoginMetaThrottleConcurrentSafe(t *testing.T) {
	th := newLoginMetaThrottle(time.Second)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			th.shouldWrite(42)
		}()
	}
	wg.Wait()
	// 首次调用应恰好有一次写库放行，其余 63 次落在窗口内被跳过
	if !th.shouldWrite(43) {
		t.Fatal("first write for a new uid must be allowed")
	}
	if th.shouldWrite(43) {
		t.Fatal("second write within TTL must be skipped")
	}
}
