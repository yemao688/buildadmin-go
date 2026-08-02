package security

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	securitymodel "go-build-admin/internal/admin/model/security"
	"go-build-admin/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm/schema"
)

func TestSecurityRuleHandlersNormalizeControllerAs(t *testing.T) {
	db, config := testutil.OpenMySQL(t)
	config.Database.Prefix = fmt.Sprintf("cas_%d_", time.Now().UnixNano())
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: config.Database.Prefix}
	dataTable := fmt.Sprintf("target_%d", time.Now().UnixNano())
	recycleTable := config.Database.Prefix + "security_data_recycle"
	sensitiveTable := config.Database.Prefix + "security_sensitive_data"
	targetTable := config.Database.Prefix + dataTable
	t.Cleanup(func() {
		db.Exec("DROP TABLE IF EXISTS `" + targetTable + "`")
		db.Exec("DROP TABLE IF EXISTS `" + recycleTable + "`")
		db.Exec("DROP TABLE IF EXISTS `" + sensitiveTable + "`")
	})
	require.NoError(t, db.Exec("CREATE TABLE `"+targetTable+"` (id BIGINT UNSIGNED NOT NULL, display_name VARCHAR(100) NOT NULL DEFAULT '', PRIMARY KEY (id))").Error)
	require.NoError(t, db.Table(recycleTable).AutoMigrate(&securitymodel.SecurityDataRecycle{}))
	require.NoError(t, db.Table(sensitiveTable).AutoMigrate(&securitymodel.SecuritySensitiveData{}))

	gin.SetMode(gin.TestMode)
	t.Run("data recycle add and edit", func(t *testing.T) {
		handler := NewDataRecycleHandler(nil, config, securitymodel.NewDataRecycleModel(db, config), nil)
		router := gin.New()
		router.POST("/add", handler.Add)
		router.POST("/edit", handler.Edit)

		addRecorder := httptest.NewRecorder()
		router.ServeHTTP(addRecorder, httptest.NewRequest(http.MethodPost, "/add", bytes.NewBufferString(fmt.Sprintf(`{"name":"test recycle","controller":"security.DataRecycle","data_table":"%s","primary_key":"id","status":"1"}`, dataTable))))
		require.Equal(t, http.StatusOK, addRecorder.Code, addRecorder.Body.String())

		var row securitymodel.SecurityDataRecycle
		require.NoError(t, db.Table(recycleTable).Where("name = ?", "test recycle").First(&row).Error)
		require.Equal(t, "security/datarecycle", row.ControllerAs)

		editRecorder := httptest.NewRecorder()
		router.ServeHTTP(editRecorder, httptest.NewRequest(http.MethodPost, "/edit", bytes.NewBufferString(fmt.Sprintf(`{"id":%d,"name":"test recycle edit","controller":"security.DataRecycle","data_table":"%s","primary_key":"id","status":"1"}`, row.ID, dataTable))))
		require.Equal(t, http.StatusOK, editRecorder.Code, editRecorder.Body.String())
		require.NoError(t, db.Table(recycleTable).Where("id = ?", row.ID).First(&row).Error)
		require.Equal(t, "security/datarecycle", row.ControllerAs)
	})

	t.Run("sensitive data add and edit", func(t *testing.T) {
		handler := NewSensitiveDataHandler(nil, config, securitymodel.NewSensitiveDataModel(db, config), nil)
		router := gin.New()
		router.POST("/add", handler.Add)
		router.POST("/edit", handler.Edit)
		fields := `[{"name":"display_name","value":"Display name"}]`

		addRecorder := httptest.NewRecorder()
		router.ServeHTTP(addRecorder, httptest.NewRequest(http.MethodPost, "/add", bytes.NewBufferString(fmt.Sprintf(`{"name":"test sensitive","controller":"security.SensitiveData","data_table":"%s","primary_key":"id","fields":%s,"status":"1"}`, dataTable, fields))))
		require.Equal(t, http.StatusOK, addRecorder.Code, addRecorder.Body.String())

		var row securitymodel.SecuritySensitiveData
		require.NoError(t, db.Table(sensitiveTable).Where("name = ?", "test sensitive").First(&row).Error)
		require.Equal(t, "security/sensitivedata", row.ControllerAs)

		editRecorder := httptest.NewRecorder()
		router.ServeHTTP(editRecorder, httptest.NewRequest(http.MethodPost, "/edit", bytes.NewBufferString(fmt.Sprintf(`{"id":%d,"name":"test sensitive edit","controller":"security.SensitiveData","data_table":"%s","primary_key":"id","fields":%s,"status":"1"}`, row.ID, dataTable, fields))))
		require.Equal(t, http.StatusOK, editRecorder.Code, editRecorder.Body.String())
		require.NoError(t, db.Table(sensitiveTable).Where("id = ?", row.ID).First(&row).Error)
		require.Equal(t, "security/sensitivedata", row.ControllerAs)
	})
}
