package repository

import (
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/header"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// securityLogFixture provides the tables and seed data needed by the log
// read paths (GetOne/List/Del stay repository-level data access).
type securityLogFixture struct {
	db     *gorm.DB
	prefix string
	config *conf.Configuration
}

func newSecurityLogFixture(t *testing.T) *securityLogFixture {
	t.Helper()
	prefix := fmt.Sprintf("sl_it_%d_", os.Getpid())
	db, config := testutil.OpenMySQL(t)
	config.Database.Prefix = prefix
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	f := &securityLogFixture{db: db, prefix: prefix, config: config}
	tables := []string{"admin", "admin_closure", "user", "security_data_recycle", "security_data_recycle_log", "security_sensitive_data", "security_sensitive_data_log"}
	for _, table := range tables {
		db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`")
	}
	t.Cleanup(func() {
		for _, table := range tables {
			db.Exec("DROP TABLE IF EXISTS `" + prefix + table + "`")
		}
		_ = sqlDB.Close()
	})
	q := func(name string) string { return "`" + prefix + name + "`" }
	for _, stmt := range []string{
		"CREATE TABLE " + q("admin") + " (id INT PRIMARY KEY, parent_id INT NULL, username VARCHAR(64) NOT NULL, nickname VARCHAR(64) NOT NULL)",
		"CREATE TABLE " + q("admin_closure") + " (ancestor_id INT NOT NULL, descendant_id INT NOT NULL, depth INT NOT NULL, PRIMARY KEY (ancestor_id,descendant_id))",
		"CREATE TABLE " + q("user") + " (id INT PRIMARY KEY, admin_id INT NOT NULL, username VARCHAR(64) NOT NULL)",
		"CREATE TABLE " + q("security_data_recycle_log") + " (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, recycle_id INT NOT NULL, data LONGTEXT NOT NULL, data_table VARCHAR(64) NOT NULL, primary_key VARCHAR(64) NOT NULL, is_restore INT NOT NULL DEFAULT 0, connection VARCHAR(64) NOT NULL DEFAULT '', ip VARCHAR(64) NOT NULL, useragent VARCHAR(255) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("security_sensitive_data_log") + " (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, sensitive_id INT NOT NULL, data_table VARCHAR(64) NOT NULL, primary_key VARCHAR(64) NOT NULL, data_field VARCHAR(64) NOT NULL, data_comment VARCHAR(255) NOT NULL, id_value INT NOT NULL, `before` TEXT NOT NULL, `after` TEXT NOT NULL, is_rollback INT NOT NULL DEFAULT 0, connection VARCHAR(64) NOT NULL DEFAULT '', ip VARCHAR(64) NOT NULL, useragent VARCHAR(255) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0)",
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}
	require.NoError(t, db.Table(prefix+"security_data_recycle").AutoMigrate(&model.SecurityDataRecycle{}))
	require.NoError(t, db.Table(prefix+"security_sensitive_data").AutoMigrate(&model.SecuritySensitiveData{}))
	require.NoError(t, db.Exec("INSERT INTO "+q("admin")+" VALUES (1,NULL,'root','Root'),(2,1,'child','Child'),(3,1,'sibling','Sibling')").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("admin_closure")+" VALUES (1,1,0),(1,2,1),(1,3,1),(2,2,0),(3,3,0)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("user")+" VALUES (20,2,'before'),(21,3,'sibling')").Error)
	require.NoError(t, db.Table(prefix+"security_data_recycle").Create(&model.SecurityDataRecycle{ID: 1, Name: "user", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", Status: "1"}).Error)
	require.NoError(t, db.Table(prefix+"security_sensitive_data").Create(&model.SecuritySensitiveData{ID: 1, Name: "user", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", DataFields: `{"username":"username"}`, Status: "1"}).Error)
	return f
}

func TestSecurityLogListsKeepJoinedAdminAndRuleFields(t *testing.T) {
	f := newSecurityLogFixture(t)
	ctx, _ := gin.CreateTestContext(nil)
	_ = data_scope.SetActor(ctx, data_scope.Actor{AdminID: 2})
	ctx.Request = httptest.NewRequest("GET", "/admin/security.DataRecycleLog/index", nil)
	header.SetAdminAuth(ctx, header.AdminAuth{Id: 1, IsSuperAdmin: true})
	q := func(n string) string { return "`" + f.prefix + n + "`" }
	require.NoError(t, f.db.Exec("INSERT INTO "+q("security_data_recycle_log")+" (admin_id,recycle_id,data,data_table,primary_key,ip,useragent) VALUES (2,1,'{}','user','id','127.0.0.1','test')").Error)
	require.NoError(t, f.db.Exec("INSERT INTO "+q("security_sensitive_data_log")+" (admin_id,sensitive_id,data_table,primary_key,data_field,data_comment,id_value,`before`,`after`,ip,useragent) VALUES (2,1,'user','id','username','username',20,'before','after','127.0.0.1','test')").Error)

	recycleList, recycleTotal, err := NewSecurityDataRecycleLogRepository(f.db, f.config).List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), recycleTotal)
	require.Len(t, recycleList, 1)
	require.Equal(t, "Child", recycleList[0].Admin.Nickname)
	require.Equal(t, "user", recycleList[0].Recycle.Name)

	sensitiveList, sensitiveTotal, err := NewSecuritySensitiveDataLogRepository(f.db, f.config).List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), sensitiveTotal)
	require.Len(t, sensitiveList, 1)
	require.Equal(t, "Child", sensitiveList[0].Admin.Nickname)
	require.Equal(t, "user", sensitiveList[0].SensitiveData.Name)
}
