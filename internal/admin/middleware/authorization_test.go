package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	adminModel "go-build-admin/internal/admin/repository/auth"
	"go-build-admin/internal/conf"
	model "go-build-admin/internal/model"
	"go-build-admin/internal/pkg/header"
	"go-build-admin/internal/utils"

	ginI18n "github.com/gin-contrib/i18n"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/text/language"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type authorizationFixture struct {
	auth *adminModel.AuthRepository
	db   *gorm.DB
}

func newAuthorizationFixture(t *testing.T) *authorizationFixture {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:admin-authorization-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AdminRule{}, &model.AdminGroup{}, &model.AdminGroupAccess{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return &authorizationFixture{
		auth: adminModel.NewAuthRepository(db, nil, &conf.Configuration{}),
		db:   db,
	}
}

func (f *authorizationFixture) addRule(t *testing.T, name string) model.AdminRule {
	t.Helper()
	rule := model.AdminRule{Type: "button", Title: name, Name: name, Status: "1"}
	require.NoError(t, f.db.Create(&rule).Error)
	return rule
}

func (f *authorizationFixture) grantRule(t *testing.T, uid int32, rules string) {
	t.Helper()
	group := model.AdminGroup{
		Name:   "group-" + strconv.Itoa(int(uid)),
		Rules:  rules,
		Status: "1",
	}
	require.NoError(t, f.db.Create(&group).Error)
	require.NoError(t, f.db.Create(&model.AdminGroupAccess{UID: uid, GroupID: group.ID}).Error)
}

func (f *authorizationFixture) request(t *testing.T, path string, uid int32) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(ginI18n.Localize(ginI18n.WithBundle(&ginI18n.BundleCfg{
		RootPath:         utils.RootPath() + "/internal/conf/localize",
		AcceptLanguage:   []language.Tag{language.English},
		DefaultLanguage:  language.English,
		UnmarshalFunc:    json.Unmarshal,
		FormatBundleFile: "json",
	})))
	router.Use(func(c *gin.Context) {
		c.Set("AdminAuth", header.AdminAuth{Id: uid})
	})
	router.GET(path, NewAuthorization(f.auth, zap.NewNop()).Handler(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func authorizationBusinessCode(t *testing.T, recorder *httptest.ResponseRecorder) int {
	t.Helper()
	var response struct {
		Code int `json:"code"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response.Code
}

func TestAuthorizationAllowsAuthorizedExistingRule(t *testing.T) {
	fixture := newAuthorizationFixture(t)
	rule := fixture.addRule(t, "country/currency/index")
	fixture.grantRule(t, 1, strconv.Itoa(int(rule.ID)))

	recorder := fixture.request(t, "/admin/country.Currency/index", 1)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestAuthorizationRejectsUnauthorizedExistingRule(t *testing.T) {
	fixture := newAuthorizationFixture(t)
	fixture.addRule(t, "country/currency/index")

	recorder := fixture.request(t, "/admin/country.Currency/index", 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusForbidden, authorizationBusinessCode(t, recorder))
}

func TestAuthorizationRejectsUnknownRule(t *testing.T) {
	fixture := newAuthorizationFixture(t)

	recorder := fixture.request(t, "/admin/country.Currency/index", 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusForbidden, authorizationBusinessCode(t, recorder))
}

func TestAuthorizationAllowsPermissionExemptRoute(t *testing.T) {
	fixture := newAuthorizationFixture(t)
	RegisterPermissionExempt("country/currency", "index")
	t.Cleanup(func() { UnregisterPermissionExempt("country/currency", "index") })

	recorder := fixture.request(t, "/admin/country.Currency/index", 1)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestAuthorizationAllowsIndexIndexPermissionExempt(t *testing.T) {
	fixture := newAuthorizationFixture(t)
	RegisterPermissionExempt("index", "index")
	t.Cleanup(func() { UnregisterPermissionExempt("index", "index") })

	recorder := fixture.request(t, "/admin/Index/index", 1)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestAuthorizationAllowsSuperAdmin(t *testing.T) {
	fixture := newAuthorizationFixture(t)
	fixture.addRule(t, "country/currency/index")
	fixture.grantRule(t, 1, "*")

	recorder := fixture.request(t, "/admin/country.Currency/index", 1)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestAuthorizationEnforcesRuleAddedAfterCacheInvalidation(t *testing.T) {
	fixture := newAuthorizationFixture(t)

	initial := fixture.request(t, "/admin/country.Currency/index", 1)
	require.Equal(t, http.StatusOK, initial.Code)
	require.Equal(t, http.StatusForbidden, authorizationBusinessCode(t, initial))

	fixture.addRule(t, "country/currency/index")
	fixture.auth.InvalidateAll()

	recorder := fixture.request(t, "/admin/country.Currency/index", 1)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, http.StatusForbidden, authorizationBusinessCode(t, recorder))
}

func TestAuthorizationAllowsNonAdminPath(t *testing.T) {
	fixture := newAuthorizationFixture(t)

	recorder := fixture.request(t, "/api/demo/index", 1)

	require.Equal(t, http.StatusNoContent, recorder.Code)
}
