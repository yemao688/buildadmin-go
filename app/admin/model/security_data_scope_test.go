package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"gorm.io/driver/mysql"
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
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BUILDADMIN_TEST_MYSQL_DSN is not set")
	}
	prefix := fmt.Sprintf("sm_it_%d_", os.Getpid())
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}, DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	db = db.Debug()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	f := &securityModelFixture{db: db, prefix: prefix, config: &conf.Configuration{Database: conf.Database{Prefix: prefix}}}
	tables := []string{"admin", "admin_closure", "user", "security_data_recycle_log", "security_sensitive_data_log"}
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
		"CREATE TABLE " + q("admin") + " (id INT PRIMARY KEY, parent_id INT NULL)",
		"CREATE TABLE " + q("admin_closure") + " (ancestor_id INT NOT NULL, descendant_id INT NOT NULL, depth INT NOT NULL, PRIMARY KEY (ancestor_id,descendant_id))",
		"CREATE TABLE " + q("user") + " (id INT PRIMARY KEY, admin_id INT NOT NULL, username VARCHAR(64) NOT NULL, birthday VARCHAR(32) NOT NULL DEFAULT '')",
		"CREATE TABLE " + q("security_data_recycle_log") + " (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, target_admin_id INT NOT NULL, recycle_id INT NOT NULL, data LONGTEXT NOT NULL, data_table VARCHAR(64) NOT NULL, primary_key VARCHAR(64) NOT NULL, is_restore INT NOT NULL DEFAULT 0, is_committed INT NOT NULL DEFAULT 0, legacy_unrecoverable INT NOT NULL DEFAULT 0, connection VARCHAR(64) NOT NULL DEFAULT '', ip VARCHAR(64) NOT NULL, useragent VARCHAR(255) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("security_sensitive_data_log") + " (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, target_admin_id INT NOT NULL, sensitive_id INT NOT NULL, data_table VARCHAR(64) NOT NULL, primary_key VARCHAR(64) NOT NULL, data_field VARCHAR(64) NOT NULL, data_comment VARCHAR(255) NOT NULL, id_value INT NOT NULL, `before` TEXT NOT NULL, `after` TEXT NOT NULL, is_rollback INT NOT NULL DEFAULT 0, is_committed INT NOT NULL DEFAULT 0, legacy_unrecoverable INT NOT NULL DEFAULT 0, connection VARCHAR(64) NOT NULL DEFAULT '', ip VARCHAR(64) NOT NULL, useragent VARCHAR(255) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0)",
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO "+q("admin")+" VALUES (1,NULL),(2,1)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("admin_closure")+" VALUES (1,1,0),(1,2,1),(2,2,0)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("user")+" VALUES (20,2,'after',''),(21,2,'after-2','')").Error)
	return f
}

func (f *securityModelFixture) context() *gin.Context {
	c, _ := gin.CreateTestContext(nil)
	_ = data_scope.SetActor(c, data_scope.Actor{AdminID: 2})
	return c
}

func TestSecurityDataScopeRestoreRollbackFailClosedAndAtomic(t *testing.T) {
	f := newSecurityModelFixture(t)
	recycle := NewDataRecycleLogModel(f.db, f.config, data_scope.NewClosureEnforcer(f.config))
	sensitive := NewSensitiveDataLogModel(f.db, f.config, data_scope.NewClosureEnforcer(f.config))
	q := func(n string) string { return "`" + f.prefix + n + "`" }
	insertRecycle := func(id, committed, legacy, owner int, data string) {
		stmt := "INSERT INTO " + q("security_data_recycle_log") + " (id,admin_id,target_admin_id,recycle_id,data,data_table,primary_key,is_committed,legacy_unrecoverable,ip,useragent) VALUES (?,?,?,?,?,'user','id',?,?,?,'test')"
		require.NoError(t, f.db.Exec(stmt, id, 2, owner, 1, data, committed, legacy, "127.0.0.1").Error)
	}
	insertRecycle(1, 0, 0, 2, `{"id":22,"admin_id":2,"username":"restored","birthday":""}`)
	require.Error(t, recycle.Restore(f.context(), []int32{1}))
	insertRecycle(2, 1, 1, 2, `{"id":23,"admin_id":2,"username":"legacy","birthday":""}`)
	require.Error(t, recycle.Restore(f.context(), []int32{2}))
	insertRecycle(3, 1, 0, 0, `{"id":24,"admin_id":2,"username":"no-owner","birthday":""}`)
	require.Error(t, recycle.Restore(f.context(), []int32{3}))
	insertRecycle(4, 1, 0, 2, `{"id":25,"admin_id":2,"username":"ok","birthday":""}`)
	insertRecycle(5, 1, 0, 2, `{"id":26,"admin_id":2,"username":"ok2","birthday":""}`)
	require.NoError(t, recycle.Restore(f.context(), []int32{4}))
	var restored int64
	f.db.Table(q("user")).Where("id=25").Count(&restored)
	require.Equal(t, int64(1), restored)
	require.Error(t, recycle.Restore(f.context(), []int32{4}))
	// A mixed batch cannot restore only its valid member.
	require.Error(t, recycle.Restore(f.context(), []int32{5, 3}))
	f.db.Table(q("user")).Where("id=26").Count(&restored)
	require.Zero(t, restored)

	insertSensitive := func(id, committed, legacy int, after string) {
		stmt := "INSERT INTO " + q("security_sensitive_data_log") + " (id,admin_id,target_admin_id,sensitive_id,data_table,primary_key,data_field,data_comment,id_value,`before`,`after`,is_committed,legacy_unrecoverable,ip,useragent) VALUES (?,?,?,?,?,'id','username','username',20,'before',?,?,?,'127.0.0.1','test')"
		require.NoError(t, f.db.Exec(stmt, id, 2, 2, 1, "user", after, committed, legacy).Error)
	}
	insertSensitive(10, 0, 0, "after")
	require.Error(t, sensitive.Rollback(f.context(), []int32{10}))
	insertSensitive(11, 1, 1, "after")
	require.Error(t, sensitive.Rollback(f.context(), []int32{11}))
	insertSensitive(12, 1, 0, "after")
	require.NoError(t, f.db.Exec("UPDATE "+q("user")+" SET username='changed' WHERE id=20").Error)
	var changed string
	require.NoError(t, f.db.Raw("SELECT username FROM "+q("user")+" WHERE id=20").Scan(&changed).Error)
	require.Equal(t, "changed", changed)
	require.Error(t, sensitive.Rollback(f.context(), []int32{12}))
	var value string
	f.db.Table(q("user")).Select("username").Where("id=20").Scan(&value)
	require.Equal(t, "changed", value)
	insertSensitive(13, 1, 0, "changed")
	insertSensitive(14, 1, 0, "other")
	require.Error(t, sensitive.Rollback(f.context(), []int32{13, 14}))
	f.db.Table(q("user")).Select("username").Where("id=20").Scan(&value)
	require.Equal(t, "changed", value)
}
