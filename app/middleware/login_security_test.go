package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	adminModel "go-build-admin/app/admin/model"
	commonModel "go-build-admin/app/common/model"
	"go-build-admin/app/pkg/token"
	"go-build-admin/conf"
	"go-build-admin/utils"
	"golang.org/x/text/language"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type loginSecurityTokenDriver struct {
	data *token.Token
}

type loginSecurityAdminRow struct {
	ID     int32  `gorm:"column:id;primaryKey"`
	Status string `gorm:"column:status"`
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
		RootPath:         utils.RootPath() + "/conf/localize",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    json.Unmarshal,
		FormatBundleFile: "json",
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
	require.NoError(t, db.Create(&loginSecurityAdminRow{ID: 1, Status: "enable"}).Error)
	driver := loginSecurityTokenDriver{data: &token.Token{Type: "user", UserID: 1}}
	authM := adminModel.NewAuthModel(db, &token.TokenHelper{Driver: driver}, &conf.Configuration{})
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
	require.NoError(t, db.Create(&loginSecurityAdminRow{ID: 1, Status: "disable"}).Error)
	driver := loginSecurityTokenDriver{data: &token.Token{Type: "admin", UserID: 1}}
	authM := adminModel.NewAuthModel(db, &token.TokenHelper{Driver: driver}, &conf.Configuration{})
	router := newLoginSecurityRouter(NewLogin(&conf.Configuration{}, &token.TokenHelper{Driver: driver}, authM).Handler())

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("batoken", "admin-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusUnauthorized, loginSecurityResponseCode(t, recorder))
}

func TestUserLoginRejectsAdminToken(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-login-security-domain?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&commonModel.User{}))
	require.NoError(t, db.Create(&commonModel.User{ID: 1, Status: "enable"}).Error)
	driver := loginSecurityTokenDriver{data: &token.Token{Type: "admin", UserID: 1}}
	authM := commonModel.NewAuthModel(db, &token.TokenHelper{Driver: driver}, &conf.Configuration{})
	router := newLoginSecurityRouter(NewUserLogin(&conf.Configuration{}, &token.TokenHelper{Driver: driver}, authM).Handler())

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
	require.NoError(t, db.AutoMigrate(&commonModel.User{}))
	require.NoError(t, db.Create(&commonModel.User{ID: 1, Status: "disable"}).Error)
	driver := loginSecurityTokenDriver{data: &token.Token{Type: "user", UserID: 1}}
	authM := commonModel.NewAuthModel(db, &token.TokenHelper{Driver: driver}, &conf.Configuration{})
	router := newLoginSecurityRouter(NewUserLogin(&conf.Configuration{}, &token.TokenHelper{Driver: driver}, authM).Handler())

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("ba-user-token", "user-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusUnauthorized, loginSecurityResponseCode(t, recorder))
}
