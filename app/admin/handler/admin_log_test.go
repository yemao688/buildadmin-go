package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-build-admin/app/admin/model"
	"go-build-admin/conf"

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
	require.NoError(t, db.AutoMigrate(&model.AdminLog{}))
	require.NoError(t, db.Create(&model.AdminLog{Username: "one"}).Error)
	require.NoError(t, db.Create(&model.AdminLog{Username: "two"}).Error)

	logModel := model.NewAdminLogModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	h := NewAdminLogHandler(nil, logModel)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/admin/auth.AdminLog/del", h.Del)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/admin/auth.AdminLog/del?ids%5B%5D=1", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 1, response.Code)

	var count int64
	require.NoError(t, db.Model(&model.AdminLog{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestAdminLogDelRejectsForgedLogFields(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:admin-log-handler-forged?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.AdminLog{}))
	require.NoError(t, db.Create(&model.AdminLog{Username: "one"}).Error)

	h := NewAdminLogHandler(nil, model.NewAdminLogModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}))
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/admin/auth.AdminLog/del?ids%5B%5D=1&username=forged", nil)
	h.Del(ctx)
	require.Equal(t, http.StatusOK, ctx.Writer.Status())
	var row model.AdminLog
	require.Error(t, db.First(&row, 1).Error)
}
