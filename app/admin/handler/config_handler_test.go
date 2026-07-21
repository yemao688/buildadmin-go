package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-build-admin/app/admin/model"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestConfigEditHandlerPersistsPostedValues(t *testing.T) {
	dsn := fmt.Sprintf("file:config-handler-edit-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: "ba_"},
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Config{}))

	config := &conf.Configuration{Database: conf.Database{Prefix: "ba_"}}
	configModel := model.NewConfigModel(db, config)
	initialGroup := `[{"key":"basics","value":"Basics"},{"key":"mail","value":"Mail"},{"key":"config_quick_entrance","value":"Config Quick entrance"},{"key":"upload","value":"Upload"}]`
	rows := []model.Config{
		{ID: 1, Name: "config_group", Type: "array", Value: initialGroup, Weigh: -1},
		{ID: 2, Name: "site_name", Type: "string", Value: "站点名称", Weigh: 99},
		{ID: 3, Name: "record_number", Type: "string", Value: "渝ICP备8888888号-1", Weigh: 0},
		{ID: 4, Name: "version", Type: "string", Value: "v1.0.0", Weigh: 0},
		{ID: 5, Name: "time_zone", Type: "string", Value: "Asia/Shanghai", Weigh: 0},
		{ID: 6, Name: "no_access_ip", Type: "textarea", Value: "", Weigh: 0},
	}
	for _, row := range rows {
		require.NoError(t, db.Table(configModel.TableName).Create(&row).Error)
	}

	h := NewConfigHandler(nil, config, configModel)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/config/edit", h.Edit)
	requestJSON := `{"site_name":"Hotel","no_access_ip":"","time_zone":"Asia/Shanghai","version":"v1.0.0","record_number":"渝ICP备8888888号-1","config_group":[{"key":"basics","value":"Basics"},{"key":"mail","value":"Mail"},{"key":"config_quick_entrance","value":"Config Quick entrance"},{"key":"upload","value":"上传配置"}]}`
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/admin/config/edit", bytes.NewBufferString(requestJSON)))

	require.Equal(t, http.StatusOK, recorder.Code)
	var response Response
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, 1, response.Code)

	values := map[string]string{}
	var persisted []model.Config
	require.NoError(t, db.Table(configModel.TableName).Find(&persisted).Error)
	for _, row := range persisted {
		values[row.Name] = row.Value
	}
	require.Equal(t, "Hotel", values["site_name"])
	require.Equal(t, "", values["no_access_ip"])
	require.Equal(t, "Asia/Shanghai", values["time_zone"])
	require.Equal(t, "v1.0.0", values["version"])
	require.Equal(t, "渝ICP备8888888号-1", values["record_number"])

	var entries []configJSONItem
	require.NoError(t, json.Unmarshal([]byte(values["config_group"]), &entries))
	require.Equal(t, []configJSONItem{
		{Key: "basics", Value: "Basics"},
		{Key: "mail", Value: "Mail"},
		{Key: "config_quick_entrance", Value: "Config Quick entrance"},
		{Key: "upload", Value: "上传配置"},
	}, entries)
}
