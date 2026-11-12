package service

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"buildadmin-go/internal/admin/dto"
	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// securityModelFixture provides the security-rule tables and seed data for
// the service-level restore/rollback and controller_as tests.
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
	require.NoError(t, db.Table(prefix+"security_data_recycle").AutoMigrate(&model.SecurityDataRecycle{}))
	require.NoError(t, db.Table(prefix+"security_sensitive_data").AutoMigrate(&model.SecuritySensitiveData{}))
	require.NoError(t, db.Exec("INSERT INTO "+q("admin")+" VALUES (1,NULL,'root','Root'),(2,1,'child','Child'),(3,1,'sibling','Sibling')").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("admin_closure")+" VALUES (1,1,0),(1,2,1),(1,3,1),(2,2,0),(3,3,0)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("user")+" VALUES (20,2,'before'),(21,3,'sibling')").Error)
	require.NoError(t, db.Table(prefix+"security_data_recycle").Create(&model.SecurityDataRecycle{ID: 1, Name: "user", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", Status: "1"}).Error)
	require.NoError(t, db.Table(prefix+"security_sensitive_data").Create(&model.SecuritySensitiveData{ID: 1, Name: "user", Controller: "user.User", ControllerAs: "user/user", DataTable: "user", PrimaryKey: "id", DataFields: `{"username":"username"}`, Status: "1"}).Error)
	return f
}

func (f *securityModelFixture) context() *gin.Context {
	c, _ := gin.CreateTestContext(nil)
	c.Request = httptest.NewRequest("POST", "/admin/security.SensitiveDataLog/rollback", nil)
	_ = data_scope.SetActor(c, data_scope.Actor{AdminID: 2})
	return c
}

func TestSecurityDataScopeRestoreRollbackFailClosedAndAtomic(t *testing.T) {
	f := newSecurityModelFixture(t)
	recycleSvc := NewSecurityDataRecycleLogService(adminmodel.NewSecurityDataRecycleLogRepository(f.db, f.config))
	sensitiveSvc := NewSecuritySensitiveDataLogService(adminmodel.NewSecuritySensitiveDataLogRepository(f.db, f.config))
	q := func(n string) string { return "`" + f.prefix + n + "`" }
	insertRecycle := func(id int, data string) {
		stmt := "INSERT INTO " + q("security_data_recycle_log") + " (id,admin_id,recycle_id,data,data_table,primary_key,ip,useragent) VALUES (?,?,?,?,'user','id','127.0.0.1','test')"
		require.NoError(t, f.db.Exec(stmt, id, 2, 1, data).Error)
	}
	insertRecycle(1, `{"id":22,"admin_id":2,"username":"restored"}`)
	require.NoError(t, recycleSvc.Restore(f.context().Request.Context(), []int32{1}))
	var restored int64
	f.db.Table(q("user")).Where("id=22").Count(&restored)
	require.Equal(t, int64(1), restored)
	require.Error(t, recycleSvc.Restore(f.context().Request.Context(), []int32{1}))

	insertRecycle(2, `{"id":23,"admin_id":3,"username":"global"}`)
	require.NoError(t, recycleSvc.Restore(f.context().Request.Context(), []int32{2}))
	f.db.Table(q("user")).Where("id=23").Count(&restored)
	require.Equal(t, int64(1), restored)

	insertRecycle(3, `not-json`)
	insertRecycle(4, `{"id":24,"admin_id":2,"username":"atomic"}`)
	// A mixed batch cannot restore only its valid member.
	require.Error(t, recycleSvc.Restore(f.context().Request.Context(), []int32{3, 4}))
	f.db.Table(q("user")).Where("id=24").Count(&restored)
	require.Zero(t, restored)

	insertSensitive := func(id int, after string) {
		stmt := "INSERT INTO " + q("security_sensitive_data_log") + " (id,admin_id,sensitive_id,data_table,primary_key,data_field,data_comment,id_value,`before`,`after`,ip,useragent) VALUES (?,?,?,'user','id','username','username',20,'before',?,'127.0.0.1','test')"
		require.NoError(t, f.db.Exec(stmt, id, 2, 1, after).Error)
	}
	require.NoError(t, f.db.Exec("UPDATE "+q("user")+" SET username='changed' WHERE id=20").Error)
	insertSensitive(10, "changed")
	require.NoError(t, sensitiveSvc.Rollback(f.context().Request.Context(), []int32{10}))
	var value string
	f.db.Table(q("user")).Select("username").Where("id=20").Scan(&value)
	require.Equal(t, "before", value)
	require.Error(t, sensitiveSvc.Rollback(f.context().Request.Context(), []int32{10}))

	insertSensitive(11, "other")
	require.Error(t, sensitiveSvc.Rollback(f.context().Request.Context(), []int32{11}))
	f.db.Table(q("user")).Select("username").Where("id=20").Scan(&value)
	require.Equal(t, "before", value)
}

func TestSecurityRuleControllerAsCannotHaveTwoEnabledRules(t *testing.T) {
	f := newSecurityModelFixture(t)
	recycleSvc := NewSecurityDataRecycleService(adminmodel.NewSecurityDataRecycleRepository(f.db, f.config))
	sensitiveSvc := NewSensitiveDataService(adminmodel.NewSecuritySensitiveDataRepository(f.db, f.config))
	ctx := f.context().Request.Context()

	require.ErrorContains(t, recycleSvc.Add(ctx, dto.SecurityDataRecycleParam{
		Name: "duplicate recycle", Controller: "user.User", DataTable: "user", PrimaryKey: "id", Status: "1",
	}), "controller_as already has an enabled security rule")
	require.NoError(t, recycleSvc.Add(ctx, dto.SecurityDataRecycleParam{
		Name: "disabled recycle", Controller: "user.User", DataTable: "user", PrimaryKey: "id", Status: "0",
	}))
	var disabledRecycle model.SecurityDataRecycle
	require.NoError(t, f.db.Table(f.prefix+"security_data_recycle").Where("name = ?", "disabled recycle").First(&disabledRecycle).Error)
	require.ErrorContains(t, recycleSvc.UpdateStatus(ctx, disabledRecycle.ID, "1"), "controller_as already has an enabled security rule")

	require.ErrorContains(t, sensitiveSvc.Add(ctx, dto.SecuritySensitiveDataParam{
		Name: "duplicate sensitive", Controller: "user.User", DataTable: "user", PrimaryKey: "id", Status: "1",
		Fields: []dto.SensitiveField{{Name: "username", Value: "username"}},
	}), "controller_as already has an enabled security rule")
	require.NoError(t, sensitiveSvc.Add(ctx, dto.SecuritySensitiveDataParam{
		Name: "disabled sensitive", Controller: "user.User", DataTable: "user", PrimaryKey: "id", Status: "0",
		Fields: []dto.SensitiveField{{Name: "username", Value: "username"}},
	}))
	// The params-based service assigns the id; reload the created rule.
	var disabledSensitive model.SecuritySensitiveData
	require.NoError(t, f.db.Table(f.prefix+"security_sensitive_data").Where("name = ?", "disabled sensitive").First(&disabledSensitive).Error)
	require.ErrorContains(t, sensitiveSvc.Edit(ctx, disabledSensitive.ID, dto.SecuritySensitiveDataParam{
		Name: "enabled sensitive", Controller: "user.User", DataTable: "user", PrimaryKey: "id", Status: "1",
		Fields: []dto.SensitiveField{{Name: "username", Value: "username"}},
	}), "controller_as already has an enabled security rule")
}

// newSecurityRuleSQLiteFixture 提供两张安全规则表的 sqlite 副本，用于验证
// 删除守卫与 repo 委托，不依赖 MySQL 门禁。
func newSecurityRuleSQLiteFixture(t *testing.T) (*gorm.DB, *conf.Configuration) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:security-rules?mode=memory&cache=shared"), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{SingularTable: true},
	})
	require.NoError(t, err)
	for _, stmt := range []string{
		"DROP TABLE IF EXISTS security_data_recycle",
		"DROP TABLE IF EXISTS security_sensitive_data",
		"CREATE TABLE security_data_recycle (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL DEFAULT '', controller TEXT NOT NULL DEFAULT '', controller_as TEXT NOT NULL DEFAULT '', data_table TEXT NOT NULL DEFAULT '', primary_key TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '1', connection TEXT NOT NULL DEFAULT '', update_time INTEGER DEFAULT NULL, create_time INTEGER DEFAULT NULL)",
		"CREATE TABLE security_sensitive_data (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL DEFAULT '', controller TEXT NOT NULL DEFAULT '', controller_as TEXT NOT NULL DEFAULT '', data_table TEXT NOT NULL DEFAULT '', primary_key TEXT NOT NULL DEFAULT '', data_fields TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT '1', connection TEXT NOT NULL DEFAULT '', update_time INTEGER DEFAULT NULL, create_time INTEGER DEFAULT NULL)",
		"INSERT INTO security_data_recycle (id,name) VALUES (1,'custom'),(5,'会员')",
		"INSERT INTO security_sensitive_data (id,name) VALUES (1,'custom'),(2,'会员数据')",
	} {
		require.NoError(t, db.Exec(stmt).Error)
	}
	config := &conf.Configuration{}
	return db, config
}

func TestSecuritySeedRuleCannotBeDeleted(t *testing.T) {
	// nil repo 也可：seed 守卫在触达 repo 之前返回。
	require.ErrorContains(t, (&SecurityDataRecycleService{}).Del(context.Background(), []int32{1, 5}), "framework seed rule cannot be deleted")
	require.ErrorContains(t, (&SensitiveDataService{}).Del(context.Background(), []int32{2}), "framework seed rule cannot be deleted")

	db, config := newSecurityRuleSQLiteFixture(t)
	recycleSvc := NewSecurityDataRecycleService(adminmodel.NewSecurityDataRecycleRepository(db, config))
	sensitiveSvc := NewSensitiveDataService(adminmodel.NewSecuritySensitiveDataRepository(db, config))
	ctx := context.Background()

	require.ErrorContains(t, recycleSvc.Del(ctx, []int32{5}), "framework seed rule cannot be deleted")
	require.NoError(t, recycleSvc.Del(ctx, []int32{1}))
	var recycleRows, sensitiveRows int64
	require.NoError(t, db.Table("security_data_recycle").Where("id = 5").Count(&recycleRows).Error)
	require.Equal(t, int64(1), recycleRows)
	require.NoError(t, db.Table("security_data_recycle").Where("id = 1").Count(&recycleRows).Error)
	require.Zero(t, recycleRows)

	require.ErrorContains(t, sensitiveSvc.Del(ctx, []int32{2}), "framework seed rule cannot be deleted")
	require.NoError(t, sensitiveSvc.Del(ctx, []int32{1}))
	require.NoError(t, db.Table("security_sensitive_data").Where("id = 2").Count(&sensitiveRows).Error)
	require.Equal(t, int64(1), sensitiveRows)
	require.NoError(t, db.Table("security_sensitive_data").Where("id = 1").Count(&sensitiveRows).Error)
	require.Zero(t, sensitiveRows)
}
