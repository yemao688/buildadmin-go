package security

import (
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/internal/pkg/data_scope"
	"go-build-admin/internal/pkg/header"
	"go-build-admin/internal/pkg/testutil"
	"go-build-admin/internal/conf"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type securityModelFixture struct {
	db     *gorm.DB
	prefix string
	config *conf.Configuration
}

func newSecurityModelFixture(t *testing.T) *securityModelFixture {
	t.Helper()
	prefix := fmt.Sprintf("sm_it_%d_", os.Getpid())
	db, config := testutil.OpenMySQL(t)
	config.Database.Prefix = prefix
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	f := &securityModelFixture{db: db, prefix: prefix, config: config}
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
	require.NoError(t, db.Table(prefix+"security_data_recycle").AutoMigrate(&SecurityDataRecycle{}))
	require.NoError(t, db.Table(prefix+"security_sensitive_data").AutoMigrate(&SecuritySensitiveData{}))
	require.NoError(t, db.Exec("INSERT INTO "+q("admin")+" VALUES (1,NULL,'root','Root'),(2,1,'child','Child'),(3,1,'sibling','Sibling')").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("admin_closure")+" VALUES (1,1,0),(1,2,1),(1,3,1),(2,2,0),(3,3,0)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("user")+" VALUES (20,2,'before'),(21,3,'sibling')").Error)
	require.NoError(t, db.Table(prefix+"security_data_recycle").Create(&SecurityDataRecycle{ID: 1, Name: "user", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", Status: "1"}).Error)
	require.NoError(t, db.Table(prefix+"security_sensitive_data").Create(&SecuritySensitiveData{ID: 1, Name: "user", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", DataFields: `{"username":"username"}`, Status: "1"}).Error)
	return f
}

func (f *securityModelFixture) context() *gin.Context {
	c, _ := gin.CreateTestContext(nil)
	_ = data_scope.SetActor(c, data_scope.Actor{AdminID: 2})
	return c
}

func TestSecurityDataScopeRestoreRollbackFailClosedAndAtomic(t *testing.T) {
	f := newSecurityModelFixture(t)
	recycle := NewDataRecycleLogModel(f.db, f.config)
	sensitive := NewSensitiveDataLogModel(f.db, f.config)
	q := func(n string) string { return "`" + f.prefix + n + "`" }
	insertRecycle := func(id int, data string) {
		stmt := "INSERT INTO " + q("security_data_recycle_log") + " (id,admin_id,recycle_id,data,data_table,primary_key,ip,useragent) VALUES (?,?,?,?,'user','id','127.0.0.1','test')"
		require.NoError(t, f.db.Exec(stmt, id, 2, 1, data).Error)
	}
	insertRecycle(1, `{"id":22,"admin_id":2,"username":"restored"}`)
	require.NoError(t, recycle.Restore(f.context(), []int32{1}))
	var restored int64
	f.db.Table(q("user")).Where("id=22").Count(&restored)
	require.Equal(t, int64(1), restored)
	require.Error(t, recycle.Restore(f.context(), []int32{1}))

	insertRecycle(2, `{"id":23,"admin_id":3,"username":"global"}`)
	require.NoError(t, recycle.Restore(f.context(), []int32{2}))
	f.db.Table(q("user")).Where("id=23").Count(&restored)
	require.Equal(t, int64(1), restored)

	insertRecycle(3, `not-json`)
	insertRecycle(4, `{"id":24,"admin_id":2,"username":"atomic"}`)
	// A mixed batch cannot restore only its valid member.
	require.Error(t, recycle.Restore(f.context(), []int32{3, 4}))
	f.db.Table(q("user")).Where("id=24").Count(&restored)
	require.Zero(t, restored)

	insertSensitive := func(id int, after string) {
		stmt := "INSERT INTO " + q("security_sensitive_data_log") + " (id,admin_id,sensitive_id,data_table,primary_key,data_field,data_comment,id_value,`before`,`after`,ip,useragent) VALUES (?,?,?,'user','id','username','username',20,'before',?,'127.0.0.1','test')"
		require.NoError(t, f.db.Exec(stmt, id, 2, 1, after).Error)
	}
	require.NoError(t, f.db.Exec("UPDATE "+q("user")+" SET username='changed' WHERE id=20").Error)
	insertSensitive(10, "changed")
	require.NoError(t, sensitive.Rollback(f.context(), []int32{10}))
	var value string
	f.db.Table(q("user")).Select("username").Where("id=20").Scan(&value)
	require.Equal(t, "before", value)
	require.Error(t, sensitive.Rollback(f.context(), []int32{10}))

	insertSensitive(11, "other")
	require.Error(t, sensitive.Rollback(f.context(), []int32{11}))
	f.db.Table(q("user")).Select("username").Where("id=20").Scan(&value)
	require.Equal(t, "before", value)
}

func TestSecurityRuleControllerAsCannotHaveTwoEnabledRules(t *testing.T) {
	f := newSecurityModelFixture(t)
	recycle := NewDataRecycleModel(f.db, f.config)
	sensitive := NewSensitiveDataModel(f.db, f.config)
	ctx := f.context()

	require.ErrorContains(t, recycle.Add(ctx, SecurityDataRecycle{
		ID: 10, Name: "duplicate recycle", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", Status: "1",
	}), "controller_as already has an enabled security rule")
	require.NoError(t, recycle.Add(ctx, SecurityDataRecycle{
		ID: 11, Name: "disabled recycle", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", Status: "0",
	}))
	require.ErrorContains(t, recycle.UpdateStatus(ctx, 11, "1"), "controller_as already has an enabled security rule")

	require.ErrorContains(t, sensitive.Add(ctx, SecuritySensitiveData{
		ID: 10, Name: "duplicate sensitive", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", DataFields: `{"username":"username"}`, Status: "1",
	}), "controller_as already has an enabled security rule")
	require.NoError(t, sensitive.Add(ctx, SecuritySensitiveData{
		ID: 11, Name: "disabled sensitive", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", DataFields: `{"username":"username"}`, Status: "0",
	}))
	require.ErrorContains(t, sensitive.Edit(ctx, SecuritySensitiveData{
		ID: 11, Name: "enabled sensitive", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", DataFields: `{"username":"username"}`, Status: "1",
	}), "controller_as already has an enabled security rule")
}

func TestSecurityLogListsKeepJoinedAdminAndRuleFields(t *testing.T) {
	f := newSecurityModelFixture(t)
	ctx := f.context()
	ctx.Request = httptest.NewRequest("GET", "/admin/security.DataRecycleLog/index", nil)
	header.SetAdminAuth(ctx, header.AdminAuth{Id: 1, IsSuperAdmin: true})
	q := func(n string) string { return "`" + f.prefix + n + "`" }
	require.NoError(t, f.db.Exec("INSERT INTO "+q("security_data_recycle_log")+" (admin_id,recycle_id,data,data_table,primary_key,ip,useragent) VALUES (2,1,'{}','user','id','127.0.0.1','test')").Error)
	require.NoError(t, f.db.Exec("INSERT INTO "+q("security_sensitive_data_log")+" (admin_id,sensitive_id,data_table,primary_key,data_field,data_comment,id_value,`before`,`after`,ip,useragent) VALUES (2,1,'user','id','username','username',20,'before','after','127.0.0.1','test')").Error)

	recycleList, recycleTotal, err := NewDataRecycleLogModel(f.db, f.config).List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), recycleTotal)
	require.Len(t, recycleList, 1)
	require.Equal(t, "Child", recycleList[0].Admin.Nickname)
	require.Equal(t, "user", recycleList[0].Recycle.Name)

	sensitiveList, sensitiveTotal, err := NewSensitiveDataLogModel(f.db, f.config).List(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), sensitiveTotal)
	require.Len(t, sensitiveList, 1)
	require.Equal(t, "Child", sensitiveList[0].Admin.Nickname)
	require.Equal(t, "user", sensitiveList[0].SensitiveData.Name)
}
