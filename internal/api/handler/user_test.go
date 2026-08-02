package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"buildadmin-go/internal/api/service/member"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/utils"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func newUserHandlerTest(config *conf.Configuration) *UserHandler {
	return &UserHandler{config: config, authM: member.NewService(nil, nil, config)}
}

func userTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("password", utils.ValidatePassword)
	}
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

func TestUserLoginRejectsInvalidAccount(t *testing.T) {
	config := &conf.Configuration{}
	config.App.UserLoginCaptcha = false
	h := newUserHandlerTest(config)
	router := userTestRouter()
	router.POST("/api/user/login", h.Login)

	request := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewBufferString(`{"username":"!","password":"Valid123!"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRegisterRequiresClickCaptcha(t *testing.T) {
	config := &conf.Configuration{}
	h := newUserHandlerTest(config)
	router := userTestRouter()
	router.POST("/api/user/register", h.Register)

	request := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(`{"username":"new_user","password":"Valid123!"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRegisterRejectsInvalidUsernameBeforeCaptcha(t *testing.T) {
	config := &conf.Configuration{}
	h := newUserHandlerTest(config)
	router := userTestRouter()
	router.POST("/api/user/register", h.Register)

	request := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewBufferString(`{"username":"bad user","password":"Valid123!","captchaId":"id","captchaInfo":"info"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusBadRequest, response.Code)
}
