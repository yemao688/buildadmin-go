package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	adminmiddleware "buildadmin-go/internal/admin/middleware"
	adminauth "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/password"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/pkg/token"
	"buildadmin-go/internal/pkg/util"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestLoginAdminLogUsesAuthenticatedAdmin(t *testing.T) {
	db, handler, record := newAdminLoginLogFixture(t)
	admin := model.Admin{
		Username: "login-admin",
		Nickname: "Login Admin",
		Password: hashLoginTestPassword(t),
		Status:   "enable",
	}
	require.NoError(t, db.Create(&admin).Error)

	router := newLoginLogRouter(handler, record)
	success := performAdminLogin(t, router, admin.Username, "correct horse battery staple")
	require.Equal(t, http.StatusOK, success.Code)

	var successLog model.AdminLog
	require.NoError(t, db.Order("id DESC").First(&successLog).Error)
	require.Equal(t, admin.ID, successLog.AdminID)
	require.Equal(t, admin.Username, successLog.Username)
	// The audit title is set on entering the login flow so both outcomes
	// carry an explicit login event instead of Unknown(login).
	require.Equal(t, "login", successLog.Title)

	failure := performAdminLogin(t, router, admin.Username, "wrong password")
	require.Equal(t, http.StatusOK, failure.Code)

	var failureLog model.AdminLog
	require.NoError(t, db.Order("id DESC").First(&failureLog).Error)
	require.Equal(t, int32(0), failureLog.AdminID)
	require.Equal(t, admin.Username, failureLog.Username)
	require.Equal(t, "login", failureLog.Title)

}

func hashLoginTestPassword(t *testing.T) string {
	t.Helper()
	hash, err := password.Hash("correct horse battery staple")
	require.NoError(t, err)
	return hash
}

func newAdminLoginLogFixture(t *testing.T) (*gorm.DB, *IndexHandler, *adminmiddleware.Record) {
	t.Helper()
	db, config := testutil.OpenMySQL(t)
	prefix := "login_log_test_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "_"
	config.Database.Prefix = prefix
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	tables := []string{"admin", "admin_log", "admin_rule", "token"}
	for _, table := range tables {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS `"+prefix+table+"`").Error)
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, table := range tables {
			_ = db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`").Error
		}
		_ = sqlDB.Close()
	})
	config.App.AdminLoginCaptcha = false
	config.App.AdminLoginRetry = 0
	config.App.AdminTokenKeepTime = 3600
	config.App.AutoWriteAdminLog = true
	config.Token.Default = "mysql"
	config.Token.Algo = "sha256"
	config.Token.Key = "admin-login-log-test-key"

	require.NoError(t, db.AutoMigrate(
		&model.Admin{},
		&model.AdminLog{},
		&model.AdminRule{},
		&token.Token{},
	))

	tokenHelper := token.NewTokenHelper(config, nil, db, nil)
	authModel := adminauth.NewAuthRepository(db, tokenHelper, config)
	logModel := adminauth.NewAdminLogRepository(db, config, authModel)
	authService := service.NewAuthService(config, authModel, nil, tokenHelper)
	return db, NewIndexHandler(config, nil, authModel, nil, nil, authService), adminmiddleware.NewRecord(config, logModel)
}

func newLoginLogRouter(handler *IndexHandler, record *adminmiddleware.Record) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         util.RootPath() + "/internal/i18n/locales",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    yaml.Unmarshal,
		FormatBundleFile: "yaml",
	})))
	router.Use(record.Handler())
	router.POST("/admin/Index/login", handler.Login)
	registerPasswordValidation()
	return router
}

func registerPasswordValidation() {
	if engine, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = engine.RegisterValidation("password", util.ValidatePassword)
	}
}

func performAdminLogin(t *testing.T, router *gin.Engine, username, password string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(Login{Username: username, Password: password})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/admin/Index/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
