package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/internal/api/service/member"
	"go-build-admin/internal/conf"
	commonModel "go-build-admin/internal/model"
	"go-build-admin/internal/pkg/testutil"
	"go-build-admin/internal/pkg/token"
	"go-build-admin/internal/utils"
	"golang.org/x/text/language"
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

func TestUserLoginWritesLastLoginFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-login?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteUserTables(db, "users", "admins"))
	user := commonModel.User{ID: 1, Username: "middleware_user", Status: "enable"}
	require.NoError(t, db.Create(&user).Error)
	authM := member.NewService(db, &token.TokenHelper{Driver: userLoginTokenDriver{}}, &conf.Configuration{})
	middleware := NewUserLogin(&conf.Configuration{}, &token.TokenHelper{Driver: userLoginTokenDriver{}}, authM)

	router := gin.New()
	router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         utils.RootPath() + "/internal/conf/localize",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    json.Unmarshal,
		FormatBundleFile: "json",
	})))
	router.Use(middleware.Handler())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("ba-user-token", "valid-token")
	request.RemoteAddr = "198.51.100.7:1234"
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	var updated commonModel.User
	require.NoError(t, db.First(&updated, 1).Error)
	require.Equal(t, "198.51.100.7", updated.LastLoginIP)
	require.Zero(t, updated.LoginFailure)
	require.NotZero(t, updated.LastLoginTime)
}
