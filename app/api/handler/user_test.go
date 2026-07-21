package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-build-admin/app/common/model"
	"go-build-admin/app/pkg/token"
	"go-build-admin/conf"
	"go-build-admin/utils"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func newUserHandlerTest(config *conf.Configuration) *UserHandler {
	return &UserHandler{
		config: config,
		authM:  model.NewAuthModel(nil, nil, config),
	}
}

func TestUserCheckInReturnsLoginCaptchaSwitch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			config := &conf.Configuration{}
			config.App.OpenMemberCenter = true
			config.App.UserLoginCaptcha = enabled
			router := gin.New()
			h := newUserHandlerTest(config)
			router.GET("/api/user/checkIn", h.CheckIn)

			request := httptest.NewRequest(http.MethodGet, "/api/user/checkIn", nil)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response Response
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			data, ok := response.Data.(map[string]interface{})
			require.True(t, ok)
			require.Equal(t, enabled, data["userLoginCaptchaSwitch"])
			require.Equal(t, []interface{}{"mobile", "email"}, data["accountVerificationType"])
		})
	}
}

func TestUserLoginSkipsCaptchaWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := &conf.Configuration{}
	config.App.OpenMemberCenter = true
	config.App.UserLoginCaptcha = false
	h := newUserHandlerTest(config)
	router := gin.New()
	router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         utils.RootPath() + "/conf/localize",
		AcceptLanguage:   []language.Tag{language.Chinese, language.TraditionalChinese, language.English},
		DefaultLanguage:  language.Chinese,
		UnmarshalFunc:    json.Unmarshal,
		FormatBundleFile: "json",
	})))
	router.POST("/api/user/checkIn", h.CheckIn)

	request := httptest.NewRequest(http.MethodPost, "/api/user/checkIn", bytes.NewBufferString(`{"tab":"login","username":"!","password":""}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	// An invalid account is rejected by Login after the disabled captcha branch is skipped.
	require.Equal(t, http.StatusOK, recorder.Code)
	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestUserRegisterCaptchaIDUsesContact(t *testing.T) {
	require.Equal(t, "person@example.comuser_register", userRegisterCaptchaID(Login{
		RegisterType: "email",
		Email:        "person@example.com",
		Mobile:       "mobile-contact",
	}))
	require.Equal(t, "mobile-contactuser_register", userRegisterCaptchaID(Login{
		RegisterType: "mobile",
		Email:        "person@example.com",
		Mobile:       "mobile-contact",
	}))
}

type userTestTokenDriver struct{}

func (userTestTokenDriver) Set(string, string, int32, int64) error { return nil }
func (userTestTokenDriver) Get(tokenString string) (*token.Token, error) {
	if tokenString == "logged-in-token" {
		return &token.Token{Token: tokenString, Type: "user", UserID: 1}, nil
	}
	return nil, errors.New("token not found")
}
func (userTestTokenDriver) Check(string, string, int32) bool { return false }
func (userTestTokenDriver) Delete(string) error              { return nil }
func (userTestTokenDriver) Clear(string, int32) error        { return nil }

func TestUserCheckInLoggedInResponseMatchesPHPContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	config := &conf.Configuration{}
	config.App.OpenMemberCenter = true
	h := newUserHandlerTest(config)
	h.authM = model.NewAuthModel(nil, &token.TokenHelper{Driver: userTestTokenDriver{}}, config)
	router := gin.New()
	router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         utils.RootPath() + "/conf/localize",
		AcceptLanguage:   []language.Tag{language.Chinese, language.TraditionalChinese, language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    json.Unmarshal,
		FormatBundleFile: "json",
	})))
	router.GET("/api/user/checkIn", h.CheckIn)

	request := httptest.NewRequest(http.MethodGet, "/api/user/checkIn", nil)
	request.Header.Set("ba-user-token", "logged-in-token")
	request.Header.Set("Accept-Language", "en")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 303, response.Code)
	require.Equal(t, "you have already logged in, no need to log in again.", response.Msg)
	data, ok := response.Data.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "logged in", data["type"])
}

func TestUserRegisterValidationRejectsInvalidInputs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("password", utils.ValidatePassword)
		_ = v.RegisterValidation("phone", utils.ValidatePhone)
	}

	base := map[string]interface{}{
		"tab":          "register",
		"registerType": "email",
		"username":     "valid_user",
		"password":     "Valid123!",
		"email":        "person@example.com",
		"captcha":      "1234",
	}
	tests := []struct {
		name   string
		mutate func(map[string]interface{})
	}{
		{"invalid tab", func(payload map[string]interface{}) { payload["tab"] = "other" }},
		{"missing register type", func(payload map[string]interface{}) { delete(payload, "registerType") }},
		{"invalid register type", func(payload map[string]interface{}) { payload["registerType"] = "username" }},
		{"mobile requires mobile", func(payload map[string]interface{}) {
			payload["registerType"] = "mobile"
			delete(payload, "mobile")
		}},
		{"mobile format invalid", func(payload map[string]interface{}) {
			payload["registerType"] = "mobile"
			payload["mobile"] = "123"
		}},
		{"email format invalid", func(payload map[string]interface{}) { payload["email"] = "invalid" }},
		{"username required", func(payload map[string]interface{}) { delete(payload, "username") }},
		{"username format invalid", func(payload map[string]interface{}) { payload["username"] = "bad user" }},
		{"username starts with digit", func(payload map[string]interface{}) { payload["username"] = "1valid" }},
		{"username contains hyphen", func(payload map[string]interface{}) { payload["username"] = "valid-name" }},
		{"username too short", func(payload map[string]interface{}) { payload["username"] = "ab" }},
		{"username too long", func(payload map[string]interface{}) { payload["username"] = "abcdefghijklmnopq" }},
		{"password required", func(payload map[string]interface{}) { delete(payload, "password") }},
		{"password invalid", func(payload map[string]interface{}) { payload["password"] = "123" }},
		{"captcha required", func(payload map[string]interface{}) { delete(payload, "captcha") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := make(map[string]interface{}, len(base))
			for key, value := range base {
				payload[key] = value
			}
			test.mutate(payload)
			body, err := json.Marshal(payload)
			require.NoError(t, err)

			config := &conf.Configuration{}
			config.App.OpenMemberCenter = true
			router := gin.New()
			router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
				RootPath:         utils.RootPath() + "/conf/localize",
				AcceptLanguage:   []language.Tag{language.Chinese, language.TraditionalChinese, language.English},
				DefaultLanguage:  language.English,
				UnmarshalFunc:    json.Unmarshal,
				FormatBundleFile: "json",
			})))
			router.POST("/api/user/checkIn", newUserHandlerTest(config).CheckIn)

			request := httptest.NewRequest(http.MethodPost, "/api/user/checkIn", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			var response Response
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}
