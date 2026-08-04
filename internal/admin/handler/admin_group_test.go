package handler

import (
	"buildadmin-go/internal/pkg/response"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	adminauth "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/admin/service"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func newAdminGroupHandlerFixture(t *testing.T) (*AdminGroupHandler, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:admin-group-handler-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)
	require.NoError(t, testutil.CreateSQLiteAdminRuleTables(db, "admin_rule", "admin_group", "admin_group_access"))
	config := &conf.Configuration{}
	groupM := adminauth.NewAdminGroupRepository(db, config)
	ruleM := adminauth.NewAdminRuleRepository(db, config)
	authM := adminauth.NewAuthRepository(db, nil, config)
	svc := service.NewAdminGroupService(groupM, ruleM, authM)
	return NewAdminGroupHandler(zap.NewNop(), groupM, ruleM, authM, svc), db
}

func quickEditRouter(t *testing.T, h *AdminGroupHandler, uid int32) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("AdminAuth", header.AdminAuth{Id: uid, IsSuperAdmin: false})
	})
	router.POST("/admin/auth.Group/edit", h.Edit)
	return router
}

func performQuickEdit(t *testing.T, router *gin.Engine, groupID int32, status string) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(map[string]any{"id": groupID, "status": status})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/admin/auth.Group/edit", bytes.NewReader(body)))
	return recorder
}

// seedQuickEditAuth seeds rules 1..3, an operator (uid 5) holding rules 1+2,
// and a target group carrying exactly rule 1 (strict subset → authorized).
func seedQuickEditAuth(t *testing.T, db *gorm.DB, targetRules string) (target model.AdminGroup) {
	t.Helper()
	for _, id := range []int32{1, 2, 3} {
		rule := model.AdminRule{ID: id, Type: "button", Title: "rule-" + strconv.Itoa(int(id)), Name: "rule-" + strconv.Itoa(int(id)), Status: "1"}
		require.NoError(t, db.Create(&rule).Error)
	}
	operator := model.AdminGroup{Name: "operator", Rules: "1,2", Status: "1"}
	require.NoError(t, db.Create(&operator).Error)
	require.NoError(t, db.Create(&model.AdminGroupAccess{UID: 5, GroupID: operator.ID}).Error)
	target = model.AdminGroup{Name: "target", Rules: targetRules, Status: "1"}
	require.NoError(t, db.Create(&target).Error)
	return target
}

func TestAdminGroupQuickEditAllowsAuthorizedOperator(t *testing.T) {
	h, db := newAdminGroupHandlerFixture(t)
	target := seedQuickEditAuth(t, db, "1")
	router := quickEditRouter(t, h, 5)

	recorder := performQuickEdit(t, router, target.ID, "0")
	var resp response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Code)

	var reloaded model.AdminGroup
	require.NoError(t, db.First(&reloaded, target.ID).Error)
	require.Equal(t, "0", reloaded.Status)
}

func TestAdminGroupQuickEditRejectsUnauthorized(t *testing.T) {
	h, db := newAdminGroupHandlerFixture(t)
	// The target group carries rule 3, outside the operator's rule set.
	target := seedQuickEditAuth(t, db, "3")
	router := quickEditRouter(t, h, 5)

	recorder := performQuickEdit(t, router, target.ID, "0")
	var resp response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.NotEqual(t, 1, resp.Code)

	var reloaded model.AdminGroup
	require.NoError(t, db.First(&reloaded, target.ID).Error)
	require.Equal(t, "1", reloaded.Status)
}

func TestAdminGroupQuickEditRejectsSelfGroup(t *testing.T) {
	h, db := newAdminGroupHandlerFixture(t)
	target := seedQuickEditAuth(t, db, "1")
	// The operator is a member of the very group it tries to switch.
	require.NoError(t, db.Create(&model.AdminGroupAccess{UID: 5, GroupID: target.ID}).Error)
	router := quickEditRouter(t, h, 5)

	recorder := performQuickEdit(t, router, target.ID, "0")
	var resp response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.NotEqual(t, 1, resp.Code)

	var reloaded model.AdminGroup
	require.NoError(t, db.First(&reloaded, target.ID).Error)
	require.Equal(t, "1", reloaded.Status)
}
