package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	commonModel "go-build-admin/app/common/model"
	"go-build-admin/app/pkg/token"
	"go-build-admin/utils"
	"golang.org/x/text/language"
)

type handlerContractTokenDriver struct {
	gotType  string
	setCount int
	get      *token.Token
	getErr   error
}

func (d *handlerContractTokenDriver) Set(_ string, typ string, _ int32, _ int64) error {
	d.gotType = typ
	d.setCount++
	return nil
}
func (d *handlerContractTokenDriver) Get(string) (*token.Token, error) { return d.get, d.getErr }
func (d *handlerContractTokenDriver) Check(string, string, int32) bool { return false }
func (d *handlerContractTokenDriver) Delete(string) error              { return nil }
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
		RootPath:         utils.RootPath() + "/conf/localize",
		AcceptLanguage:   []language.Tag{language.Chinese, language.TraditionalChinese, language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    json.Unmarshal,
		FormatBundleFile: "json",
	})))
	return router
}

func TestIndexRequiredLoginIncludesPHPType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	driver := &handlerContractTokenDriver{getErr: errors.New("not logged in")}
	h := &IndexHandler{
		authM: commonModel.NewAuthModel(nil, &token.TokenHelper{Driver: driver}, nil),
	}
	router := newContractTestRouter()
	router.GET("/", h.Index)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/?requiredLogin=1", nil))

	response := decodeHandlerResponse(t, recorder)
	require.Equal(t, 303, response.Code)
	data, ok := response.Data.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "need login", data["type"])
}

func TestEmsSendRejectsUnknownEventBeforeExternalServices(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &EmsHandler{}
	router := newContractTestRouter()
	router.POST("/", h.Send)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":"person@example.com","event":"unknown","captchaId":"id","captchaInfo":"info"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	response := decodeHandlerResponse(t, recorder)
	require.Equal(t, 400, response.Code)
	require.Equal(t, "event invalid", response.Msg)
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
