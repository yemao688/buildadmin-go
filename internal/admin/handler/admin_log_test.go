package handler

import (
	"buildadmin-go/internal/pkg/response"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	model "buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestAdminLogDel(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-log-handler?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture table with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteAdminLogTable(db, "ba_admin_log"))
	require.NoError(t, db.Create(&model.AdminLog{Username: "one"}).Error)
	require.NoError(t, db.Create(&model.AdminLog{Username: "two"}).Error)

	logModel := adminmodel.NewAdminLogRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, nil)
	h := NewAdminLogHandler(nil, logModel)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/admin/auth.AdminLog/del", h.Del)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/admin/auth.AdminLog/del?ids%5B%5D=1", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	var resp response.Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, 1, resp.Code)

	var count int64
	require.NoError(t, db.Model(&model.AdminLog{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestAdminLogDelRejectsForgedLogFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-log-handler-forged?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	require.NoError(t, err)
	// The entity carries MySQL-specific type tags (int unsigned, enum);
	// sqlite cannot AutoMigrate them, so create the fixture table with
	// sqlite-native DDL matching the runtime column shape.
	require.NoError(t, testutil.CreateSQLiteAdminLogTable(db, "ba_admin_log"))
	require.NoError(t, db.Create(&model.AdminLog{Username: "one"}).Error)

	h := NewAdminLogHandler(nil, adminmodel.NewAdminLogRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}, nil))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/admin/auth.AdminLog/del?ids%5B%5D=1&username=forged", nil)
	h.Del(ctx)
	require.Equal(t, http.StatusOK, ctx.Writer.Status())
	var row model.AdminLog
	require.Error(t, db.First(&row, 1).Error)
}
