package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	adminModel "buildadmin-go/internal/admin/repository"
	middlewarecore "buildadmin-go/internal/api/middleware"
	"buildadmin-go/internal/api/service"
	"buildadmin-go/internal/conf"
	commonModel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/header"
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

type loginSecurityTokenDriver struct {
	data *token.Token
}

type loginSecurityAdminRow struct {
	ID       int32  `gorm:"column:id;primaryKey"`
	Username string `gorm:"column:username"`
	Status   string `gorm:"column:status"`
}

func (loginSecurityAdminRow) TableName() string { return "admins" }

func (d loginSecurityTokenDriver) Set(string, string, int32, int64) error { return nil }
func (d loginSecurityTokenDriver) Get(string) (*token.Token, error)       { return d.data, nil }
func (d loginSecurityTokenDriver) Check(string, string, int32) bool       { return false }
func (d loginSecurityTokenDriver) Delete(string) error                    { return nil }
func (d loginSecurityTokenDriver) Clear(string, int32) error              { return nil }

func newLoginSecurityRouter(handler gin.HandlerFunc) *gin.Engine {
	router := gin.New()
	router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         util.RootPath() + "/internal/i18n/locales",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
	})))
	router.Use(handler)
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return router
}

func loginSecurityResponseCode(t *testing.T, recorder *httptest.ResponseRecorder) int {
	t.Helper()
	var response struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response.Code
}

func TestLoginRejectsUserToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-login-security-domain?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&loginSecurityAdminRow{}))
	require.NoError(t, db.Create(&loginSecurityAdminRow{ID: 1, Username: "enabled-admin", Status: "enable"}).Error)
	driver := loginSecurityTokenDriver{data: &token.Token{Type: "user", UserID: 1}}
	authM := adminModel.NewAuthRepository(db, &token.TokenHelper{Driver: driver}, &conf.Configuration{})
	router := newLoginSecurityRouter(NewLogin(&conf.Configuration{}, &token.TokenHelper{Driver: driver}, authM).Handler())

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("batoken", "user-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusUnauthorized, loginSecurityResponseCode(t, recorder))
}

func TestLoginRejectsDisabledAdminToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-login-security-status?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&loginSecurityAdminRow{}))
	require.NoError(t, db.Create(&loginSecurityAdminRow{ID: 1, Username: "disabled-admin", Status: "disable"}).Error)
	driver := loginSecurityTokenDriver{data: &token.Token{Type: "admin", UserID: 1}}
	authM := adminModel.NewAuthRepository(db, &token.TokenHelper{Driver: driver}, &conf.Configuration{})
	router := newLoginSecurityRouter(NewLogin(&conf.Configuration{}, &token.TokenHelper{Driver: driver}, authM).Handler())

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("batoken", "admin-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusUnauthorized, loginSecurityResponseCode(t, recorder))
}

func TestLoginStoresAuthenticatedAdminUsername(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-login-security-username?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&loginSecurityAdminRow{}))
	require.NoError(t, db.Create(&loginSecurityAdminRow{ID: 1, Username: "enabled-admin", Status: "enable"}).Error)
	driver := loginSecurityTokenDriver{data: &token.Token{Type: "admin", UserID: 1}}
	authM := adminModel.NewAuthRepository(db, &token.TokenHelper{Driver: driver}, &conf.Configuration{})
	router := newLoginSecurityRouter(NewLogin(&conf.Configuration{}, &token.TokenHelper{Driver: driver}, authM).Handler())
	router.GET("/check", func(c *gin.Context) {
		c.String(http.StatusOK, header.GetAdminAuth(c).Username)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/check", nil)
	request.Header.Set("batoken", "admin-token")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "enabled-admin", recorder.Body.String())
}

func TestUserLoginRejectsAdminToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-login-security-domain?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteUserTables(db, "users", "admins"))
	require.NoError(t, db.Create(&commonModel.User{ID: 1, Status: "enable"}).Error)
	driver := loginSecurityTokenDriver{data: &token.Token{Type: "admin", UserID: 1}}
	authM := service.NewMemberService(db, &token.TokenHelper{Driver: driver}, &conf.Configuration{})
	router := newLoginSecurityRouter(middlewarecore.NewUserLogin(&conf.Configuration{}, &token.TokenHelper{Driver: driver}, authM).Handler())

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("ba-user-token", "admin-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusUnauthorized, loginSecurityResponseCode(t, recorder))
}

func TestUserLoginRejectsDisabledUserToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-login-security-status?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteUserTables(db, "users", "admins"))
	require.NoError(t, db.Create(&commonModel.User{ID: 1, Status: "disable"}).Error)
	driver := loginSecurityTokenDriver{data: &token.Token{Type: "user", UserID: 1}}
	authM := service.NewMemberService(db, &token.TokenHelper{Driver: driver}, &conf.Configuration{})
	router := newLoginSecurityRouter(middlewarecore.NewUserLogin(&conf.Configuration{}, &token.TokenHelper{Driver: driver}, authM).Handler())

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("ba-user-token", "user-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusUnauthorized, loginSecurityResponseCode(t, recorder))
}
