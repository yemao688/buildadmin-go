package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"buildadmin-go/internal/api/service"
	"buildadmin-go/internal/conf"
	commonModel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/pkg/token"
	"buildadmin-go/internal/pkg/util"
	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type userLoginTokenDriver struct{}

func (userLoginTokenDriver) Set(string, string, int32, int64) error { return nil }
func (userLoginTokenDriver) Get(string) (*token.Token, error) {
	return &token.Token{Type: "user", UserID: 1}, nil
}
func (userLoginTokenDriver) Check(string, string, int32) bool { return false }
func (userLoginTokenDriver) Delete(string) error              { return nil }
func (userLoginTokenDriver) Clear(string, int32) error        { return nil }

// newUserLoginRouter 装配与生产一致的 gin 引擎：i18n + UserLogin 中间件 +
// 探活路由（userLoginTokenDriver 恒返回 UserID=1 的有效 user token）。
func newUserLoginRouter(authM *service.MemberService) *gin.Engine {
	middleware := NewUserLogin(&conf.Configuration{}, &token.TokenHelper{Driver: userLoginTokenDriver{}}, authM)
	router := gin.New()
	router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         util.RootPath() + "/internal/i18n/locales",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
	})))
	router.Use(middleware.Handler())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router
}

func serveUserLoginRequest(t *testing.T, router *gin.Engine, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("ba-user-token", "valid-token")
	request.RemoteAddr = remoteAddr
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestUserLoginWritesLastLoginFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-login?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteUserTables(db, "users", "admins"))
	user := commonModel.User{ID: 1, Username: "middleware_user", Status: "enable"}
	require.NoError(t, db.Create(&user).Error)
	authM := service.NewMemberService(db, &token.TokenHelper{Driver: userLoginTokenDriver{}}, &conf.Configuration{})

	recorder := serveUserLoginRequest(t, newUserLoginRouter(authM), "198.51.100.7:1234")

	require.Equal(t, http.StatusNoContent, recorder.Code)
	var updated commonModel.User
	require.NoError(t, db.First(&updated, 1).Error)
	require.Equal(t, "198.51.100.7", updated.LastLoginIP)
	require.Zero(t, updated.LoginFailure)
	require.NotZero(t, updated.LastLoginTime)
}

// TestUserLoginRejectsDisabledUser 覆盖 H6a 合并后的行为保持：被禁用会员的
// 已认证请求与旧 IsEnabledUser 分支一致地返回 code=401 "Please login first"，
// 且不写 login 元信息。
func TestUserLoginRejectsDisabledUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-login-disabled?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteUserTables(db, "users", "admins"))
	user := commonModel.User{ID: 1, Username: "disabled_user", Status: "disable"}
	require.NoError(t, db.Create(&user).Error)
	authM := service.NewMemberService(db, &token.TokenHelper{Driver: userLoginTokenDriver{}}, &conf.Configuration{})

	recorder := serveUserLoginRequest(t, newUserLoginRouter(authM), "198.51.100.7:1234")

	// AbortLogin 语义：HTTP 200 + body code=401 + "Please login first"
	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, float64(http.StatusUnauthorized), body["code"])
	require.Equal(t, "Please login first", body["msg"])
	var untouched commonModel.User
	require.NoError(t, db.First(&untouched, 1).Error)
	require.Zero(t, untouched.LastLoginTime)
	require.Empty(t, untouched.LastLoginIP)
}

// TestUserLoginRejectsMissingUser 覆盖 H6a 合并后的行为保持：token 有效但
// 账号不存在的已认证请求同样返回 code=401 "Please login first"。
func TestUserLoginRejectsMissingUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-login-missing?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteUserTables(db, "users", "admins"))
	authM := service.NewMemberService(db, &token.TokenHelper{Driver: userLoginTokenDriver{}}, &conf.Configuration{})

	recorder := serveUserLoginRequest(t, newUserLoginRouter(authM), "198.51.100.7:1234")

	require.Equal(t, http.StatusOK, recorder.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, float64(http.StatusUnauthorized), body["code"])
	require.Equal(t, "Please login first", body["msg"])
}
