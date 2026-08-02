package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/api/service/member"
	"buildadmin-go/internal/conf"
	commonmodel "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/pkg/token"
	"buildadmin-go/internal/utils"
	"golang.org/x/text/language"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type handlerContractTokenDriver struct {
	gotType  string
	setCount int
	expires  []int64
	deleted  int
	get      *token.Token
	getErr   error
}

func (d *handlerContractTokenDriver) Set(_ string, typ string, _ int32, expire int64) error {
	d.gotType = typ
	d.setCount++
	d.expires = append(d.expires, expire)
	return nil
}
func (d *handlerContractTokenDriver) Get(string) (*token.Token, error) { return d.get, d.getErr }
func (d *handlerContractTokenDriver) Check(string, string, int32) bool { return false }
func (d *handlerContractTokenDriver) Delete(string) error              { d.deleted++; return nil }
func (d *handlerContractTokenDriver) Clear(string, int32) error        { return nil }

func decodeHandlerResponse(t *testing.T, recorder *httptest.ResponseRecorder) Response {
	t.Helper()
	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func newContractTestRouter() *gin.Engine {
	router := gin.New()
	router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         utils.RootPath() + "/internal/conf/localize",
		AcceptLanguage:   []language.Tag{language.Chinese, language.TraditionalChinese, language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    json.Unmarshal,
		FormatBundleFile: "json",
	})))
	return router
}

func TestRefreshTokenRejectsUnknownTypeWithoutCreatingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	driver := &handlerContractTokenDriver{get: &token.Token{Type: "user", UserID: 1}}
	h := &CommonHandler{tokenHelper: &token.TokenHelper{Driver: driver}}
	router := newContractTestRouter()
	router.POST("/", h.RefreshToken)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"refreshToken":"refresh"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	response := decodeHandlerResponse(t, recorder)
	require.Equal(t, 400, response.Code)
	require.Equal(t, "Invalid Token!", response.Msg)
	require.Zero(t, driver.setCount)
}

func TestRefreshTokenUsesConfiguredTTLWithoutDeletingOldToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	driver := &handlerContractTokenDriver{get: &token.Token{Type: "user-refresh", UserID: 1}}
	config := &conf.Configuration{}
	config.App.UserTokenKeepTime = 259200
	db, err := gorm.Open(sqlite.Open("file:refresh-contract?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture tables with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteUserTables(db, "users", "admins"))
	require.NoError(t, db.Create(&commonmodel.User{ID: 1, Status: "enable"}).Error)
	tokenHelper := &token.TokenHelper{Driver: driver}
	h := &CommonHandler{tokenHelper: tokenHelper, authM: member.NewService(db, tokenHelper, config), config: config}
	router := newContractTestRouter()
	router.POST("/", h.RefreshToken)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"refreshToken":"refresh"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("ba-user-token", "old-token")
	router.ServeHTTP(recorder, request)

	response := decodeHandlerResponse(t, recorder)
	require.Equal(t, 1, response.Code)
	require.Equal(t, []int64{259200}, driver.expires)
	require.Zero(t, driver.deleted)
}

func TestRefreshAdminTokenUsesAdminTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	driver := &handlerContractTokenDriver{get: &token.Token{Type: "admin-refresh", UserID: 1}}
	config := &conf.Configuration{}
	config.App.AdminTokenKeepTime = 86400
	h := &CommonHandler{tokenHelper: &token.TokenHelper{Driver: driver}, config: config}
	router := newContractTestRouter()
	router.POST("/", h.RefreshToken)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"refreshToken":"refresh"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("batoken", "old-token")
	router.ServeHTTP(recorder, request)

	response := decodeHandlerResponse(t, recorder)
	require.Equal(t, 1, response.Code)
	require.Equal(t, "admin", driver.gotType)
	require.Equal(t, []int64{86400}, driver.expires)
}

func TestRefreshTokenRejectsWrongDomainHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	driver := &handlerContractTokenDriver{get: &token.Token{Type: "user-refresh", UserID: 1}}
	h := &CommonHandler{tokenHelper: &token.TokenHelper{Driver: driver}, config: &conf.Configuration{}}
	router := newContractTestRouter()
	router.POST("/", h.RefreshToken)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"refreshToken":"refresh"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("batoken", "admin-token")
	router.ServeHTTP(recorder, request)

	response := decodeHandlerResponse(t, recorder)
	require.Equal(t, 400, response.Code)
	require.Equal(t, "Invalid Token!", response.Msg)
	require.Zero(t, driver.setCount)
}
