package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	commonModel "go-build-admin/app/common/model"
	"go-build-admin/app/pkg/captcha"
	"go-build-admin/app/pkg/header"
	"go-build-admin/app/pkg/token"
	"go-build-admin/conf"
	"go-build-admin/utils"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAccountHandlerTestRouter() *gin.Engine {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("phone", utils.ValidatePhone)
		_ = v.RegisterValidation("password", utils.ValidatePassword)
	}
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

func newAccountHandlerTestDB(t *testing.T, withCaptcha bool) *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file:account-handler-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&commonModel.User{}))
	if withCaptcha {
		require.NoError(t, db.Exec(`CREATE TABLE captchas (
		key TEXT PRIMARY KEY,
		code TEXT,
		captcha TEXT,
		create_time INTEGER,
		expire_time INTEGER
	)`).Error)
	}
	return db
}

func serveJSON(t *testing.T, router *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	requestBody, err := json.Marshal(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestRetrievePasswordUsesAccountForLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name    string
		typ     string
		account string
	}{{"email", "email", "person@example.com"}, {"mobile", "mobile", "13800138000"}} {
		t.Run(test.name, func(t *testing.T) {
			db := newAccountHandlerTestDB(t, true)
			require.NoError(t, db.Create(&commonModel.User{Email: test.account, Mobile: test.account}).Error)

			h := &AccountHandler{
				userM:   commonModel.NewUserModel(db, nil),
				captcha: captcha.NewCaptcha(db),
			}
			router := newAccountHandlerTestRouter()
			router.POST("/retrieve", h.RetrievePassword)

			body := map[string]string{
				"type": test.typ, "account": test.account,
				"captcha": "1234", "password": "Password123!",
			}
			requestBody, err := json.Marshal(body)
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/retrieve", bytes.NewReader(requestBody))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusOK, recorder.Code)
			var response Response
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Equal(t, "Please enter the correct Captcha!", response.Msg)
		})
	}
}

func TestRetrievePasswordRejectsUnknownType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AccountHandler{}
	router := newAccountHandlerTestRouter()
	router.POST("/retrieve", h.RetrievePassword)

	request := httptest.NewRequest(http.MethodPost, "/retrieve", bytes.NewBufferString(`{"type":"username","account":"user","captcha":"1234","password":"Password123!"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Equal(t, "type invalid", response.Msg)
}

func TestChangeBindChecksEmailAndMobileOccupancy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		typ      string
		oldValue string
		target   string
	}{
		{"email", "old@example.com", "taken@example.com"},
		{"mobile", "13800138001", "13800138002"},
	} {
		for _, occupied := range []bool{true, false} {
			t.Run(test.typ+"/"+strconv.FormatBool(occupied), func(t *testing.T) {
				db := newAccountHandlerTestDB(t, true)
				current := commonModel.User{Email: test.oldValue, Mobile: test.oldValue}
				require.NoError(t, db.Create(&current).Error)
				if occupied {
					other := commonModel.User{Email: test.target, Mobile: test.target}
					require.NoError(t, db.Create(&other).Error)
				}

				config := &conf.Configuration{}
				config.Token.Algo = "md5"
				config.Token.Key = "test-key"
				tokenHelper := token.NewTokenHelper(config, nil, db, nil)
				require.NoError(t, db.AutoMigrate(&token.Token{}))
				verificationToken := "verification-token"
				require.NoError(t, tokenHelper.Set(verificationToken, test.typ+"-pass", current.ID, 600))
				captchaModel := captcha.NewCaptcha(db)
				captchaCode, err := captchaModel.Create(test.target + "user_change_" + test.typ)
				require.NoError(t, err)

				h := &AccountHandler{
					authM:   commonModel.NewAuthModel(db, tokenHelper, config),
					userM:   commonModel.NewUserModel(db, nil),
					captcha: captchaModel,
				}
				router := newAccountHandlerTestRouter()
				router.POST("/change-bind", func(ctx *gin.Context) {
					ctx.Set("UserAuth", header.UserAuth{Id: current.ID})
					h.ChangeBind(ctx)
				})
				body := map[string]string{
					"type": test.typ, "captcha": captchaCode,
					"accountVerificationToken": verificationToken,
				}
				if test.typ == "email" {
					body["email"] = test.target
				} else {
					body["mobile"] = test.target
				}

				recorder := serveJSON(t, router, "/change-bind", body)
				var response Response
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				if occupied {
					require.Equal(t, http.StatusBadRequest, response.Code)
				} else {
					require.Equal(t, 1, response.Code)
					var updated commonModel.User
					require.NoError(t, db.First(&updated, current.ID).Error)
					if test.typ == "email" {
						require.Equal(t, test.target, updated.Email)
					} else {
						require.Equal(t, test.target, updated.Mobile)
					}
				}
			})
		}
	}
}

func TestProfileAllowsEmptyBirthday(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newAccountHandlerTestDB(t, false)
	user := commonModel.User{Username: "olduser", Nickname: "Old", Email: "old@example.com"}
	require.NoError(t, db.Create(&user).Error)
	h := &AccountHandler{userM: commonModel.NewUserModel(db, nil)}
	router := newAccountHandlerTestRouter()
	router.POST("/profile", func(ctx *gin.Context) {
		ctx.Set("UserAuth", header.UserAuth{Id: user.ID})
		h.Profile(ctx)
	})

	recorder := serveJSON(t, router, "/profile", map[string]any{
		"username": "newuser", "nickname": "New", "gender": 0, "birthday": "", "motto": "",
	})
	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 1, response.Code)
}
