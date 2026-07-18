package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/app/admin/model"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/app/pkg/requesttx"
	"go-build-admin/conf"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type securityFixture struct {
	db     *gorm.DB
	sqlDB  interface{ Close() error }
	prefix string
	config *conf.Configuration
	root   int32
}

func newSecurityFixture(t *testing.T) *securityFixture {
	t.Helper()
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BUILDADMIN_TEST_MYSQL_DSN is not set")
	}
	prefix := fmt.Sprintf("it_%d_", os.Getpid())
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{NamingStrategy: schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}, DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	f := &securityFixture{db: db, sqlDB: sqlDB, prefix: prefix, root: 1}
	f.config = &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	tables := []string{"admin", "admin_closure", "security_data_recycle", "security_data_recycle_log", "security_sensitive_data", "security_sensitive_data_log", "user"}
	for _, name := range tables {
		db.Exec("DROP TABLE IF EXISTS `" + prefix + name + "`")
	}
	t.Cleanup(func() {
		for _, name := range tables {
			db.Exec("DROP TRIGGER IF EXISTS `" + prefix + "fail_recycle_log`")
			db.Exec("DROP TRIGGER IF EXISTS `" + prefix + "fail_sensitive_log`")
			db.Exec("DROP TABLE IF EXISTS `" + prefix + name + "`")
		}
		_ = sqlDB.Close()
	})
	q := func(name string) string { return "`" + prefix + name + "`" }
	stmts := []string{
		"CREATE TABLE " + q("admin") + " (id INT PRIMARY KEY, parent_id INT NULL, username VARCHAR(64) NOT NULL)",
		"CREATE TABLE " + q("admin_closure") + " (ancestor_id INT NOT NULL, descendant_id INT NOT NULL, depth INT NOT NULL, PRIMARY KEY (ancestor_id, descendant_id))",
		"CREATE TABLE " + q("user") + " (id INT PRIMARY KEY, admin_id INT NOT NULL, name VARCHAR(64) NOT NULL, username VARCHAR(64) NOT NULL, birthday VARCHAR(32) NOT NULL DEFAULT '', value INT NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("security_data_recycle") + " (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, name VARCHAR(64) NOT NULL, controller VARCHAR(64) NOT NULL, controller_as VARCHAR(64) NOT NULL, data_table VARCHAR(64) NOT NULL, owner_column VARCHAR(64) NOT NULL DEFAULT 'admin_id', primary_key VARCHAR(64) NOT NULL, status VARCHAR(8) NOT NULL, connection VARCHAR(64) NOT NULL DEFAULT '', update_time BIGINT NOT NULL DEFAULT 0, create_time BIGINT NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("security_data_recycle_log") + " (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, target_admin_id INT NOT NULL, recycle_id INT NOT NULL, data LONGTEXT NOT NULL, data_table VARCHAR(64) NOT NULL, primary_key VARCHAR(64) NOT NULL, is_restore INT NOT NULL DEFAULT 0, is_committed INT NOT NULL DEFAULT 0, connection VARCHAR(64) NOT NULL DEFAULT '', ip VARCHAR(64) NOT NULL, useragent VARCHAR(255) NOT NULL, create_time BIGINT NOT NULL DEFAULT 0, legacy_unrecoverable INT NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("security_sensitive_data") + " (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, name VARCHAR(64) NOT NULL, controller VARCHAR(64) NOT NULL, controller_as VARCHAR(64) NOT NULL, data_table VARCHAR(64) NOT NULL, owner_column VARCHAR(64) NOT NULL DEFAULT 'admin_id', primary_key VARCHAR(64) NOT NULL, data_fields TEXT NOT NULL, status VARCHAR(8) NOT NULL, connection VARCHAR(64) NOT NULL DEFAULT '', update_time BIGINT NOT NULL DEFAULT 0, create_time BIGINT NOT NULL DEFAULT 0)",
		"CREATE TABLE " + q("security_sensitive_data_log") + " (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, target_admin_id INT NOT NULL, sensitive_id INT NOT NULL, data_table VARCHAR(64) NOT NULL, primary_key VARCHAR(64) NOT NULL, data_field VARCHAR(64) NOT NULL, data_comment VARCHAR(255) NOT NULL, id_value INT NOT NULL, `before` TEXT NOT NULL, `after` TEXT NOT NULL, ip VARCHAR(64) NOT NULL, useragent VARCHAR(255) NOT NULL, is_rollback INT NOT NULL DEFAULT 0, is_committed INT NOT NULL DEFAULT 0, connection VARCHAR(64) NOT NULL DEFAULT '', create_time BIGINT NOT NULL DEFAULT 0, legacy_unrecoverable INT NOT NULL DEFAULT 0)",
	}
	for _, stmt := range stmts {
		require.NoError(t, db.Exec(stmt).Error)
	}
	require.NoError(t, db.Exec("INSERT INTO "+q("admin")+" VALUES (1,NULL,'root'),(2,1,'self'),(3,1,'sibling'),(4,2,'child')").Error)
	closure := "(1,1,0),(1,2,1),(1,3,1),(1,4,2),(2,2,0),(2,4,1),(3,3,0),(4,4,0)"
	require.NoError(t, db.Exec("INSERT INTO "+q("admin_closure")+" (ancestor_id,descendant_id,depth) VALUES "+closure).Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("user")+" VALUES (10,2,'owned','old','',1),(11,4,'child','old-child','',2),(12,3,'sibling','old-sibling','',3)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("security_data_recycle")+" (admin_id,name,controller,controller_as,data_table,primary_key,status) VALUES (1,'root-items','items','auth/admin','user','id','1'),(2,'child-items','items','auth/admin','user','id','1')").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q("security_sensitive_data")+" (admin_id,name,controller,controller_as,data_table,primary_key,data_fields,status) VALUES (1,'root-items','items','auth/admin','user','id','{\"username\":\"username\"}','1'),(2,'child-items','items','auth/admin','user','id','{\"username\":\"username\"}','1')").Error)
	return f
}

func (f *securityFixture) table(name string) string { return "`" + f.prefix + name + "`" }
func (f *securityFixture) actorContext(actor int32, unrestricted bool) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	_ = data_scope.SetActor(c, data_scope.Actor{AdminID: actor, Unrestricted: unrestricted})
	return c
}
func (f *securityFixture) router(actor int32, unrestricted bool, method string, handler gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	s := NewSecurity(f.config, zap.NewNop(), f.db, data_scope.NewClosureEnforcer(f.config))
	path := "/admin/auth.Admin/" + map[string]string{http.MethodDelete: "del", http.MethodPost: "edit"}[method]
	r.Use(func(c *gin.Context) {
		_ = data_scope.SetActor(c, data_scope.Actor{AdminID: actor, Unrestricted: unrestricted})
	}, s.Handler())
	r.Handle(method, path, handler)
	return r
}
func stage(c *gin.Context, code int, msg string) {
	if !requesttx.Stage(c.Request.Context(), requesttx.Outcome{HTTPCode: http.StatusOK, BusinessCode: code, Message: msg}) {
		panic("failed to stage request outcome")
	}
}
func deleteRequested(t *testing.T, c *gin.Context, prefix string) {
	t.Helper()
	for _, raw := range c.QueryArray("ids[]") {
		id, err := strconv.Atoi(raw)
		require.NoError(t, err)
		require.NoError(t, requesttx.DB(c.Request.Context()).Exec("DELETE FROM `"+prefix+"user` WHERE id=?", id).Error)
	}
}
func (f *securityFixture) request(t *testing.T, r *gin.Engine, method, url, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader
	if body == "" {
		rd = strings.NewReader("")
	} else {
		rd = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, url, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	r.ServeHTTP(rec, req)
	return rec
}

func TestSecurityMySQLScopeInheritanceAndAtomicDelete(t *testing.T) {
	f := newSecurityFixture(t)
	q := f.table("user")
	for _, tc := range []struct {
		name         string
		actor        int32
		unrestricted bool
		ids          string
		want         int
	}{
		{"self", 2, false, "10", http.StatusOK}, {"ancestor", 2, false, "11", http.StatusOK}, {"sibling", 2, false, "12", http.StatusForbidden}, {"unrestricted", 99, true, "12", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f.db.Exec("DELETE FROM " + q + " WHERE id=12")
			if tc.name == "sibling" || tc.name == "unrestricted" {
				f.db.Exec("INSERT IGNORE INTO " + q + " VALUES (12,3,'sibling','old-sibling','',3)")
			}
			r := f.router(tc.actor, tc.unrestricted, http.MethodDelete, func(c *gin.Context) {
				deleteRequested(t, c, f.prefix)
				stage(c, 1, "ok")
			})
			rec := f.request(t, r, http.MethodDelete, "/admin/auth.Admin/del?ids[]="+tc.ids, "")
			require.Equal(t, tc.want, rec.Code)
		})
	}
	// Mixed IDs are checked as a set before either row is deleted.
	f.db.Exec("INSERT IGNORE INTO " + q + " VALUES (10,2,'owned','old','',1),(12,3,'sibling','old-sibling','',3)")
	r := f.router(2, false, http.MethodDelete, func(c *gin.Context) { stage(c, 1, "ok") })
	rec := f.request(t, r, http.MethodDelete, "/admin/auth.Admin/del?ids[]=10&ids[]=12", "")
	require.Equal(t, http.StatusForbidden, rec.Code)
	var count int64
	require.NoError(t, f.db.Table(q).Where("id IN (10,12)").Count(&count).Error)
	require.Equal(t, int64(2), count)
}

func TestSecurityMySQLRootRuleInheritedByChild(t *testing.T) {
	f := newSecurityFixture(t)
	q := f.table("user")
	// Remove the child-owned rules so this request can only use the root rule.
	require.NoError(t, f.db.Exec("DELETE FROM "+f.table("security_data_recycle")+" WHERE admin_id=2").Error)
	require.NoError(t, f.db.Exec("DELETE FROM "+f.table("security_sensitive_data")+" WHERE admin_id=2").Error)

	deleteRouter := f.router(2, false, http.MethodDelete, func(c *gin.Context) {
		deleteRequested(t, c, f.prefix)
		stage(c, 1, "ok")
	})
	rec := f.request(t, deleteRouter, http.MethodDelete, "/admin/auth.Admin/del?ids[]=10", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var logs int64
	require.NoError(t, f.db.Table(f.table("security_data_recycle_log")).Where("data_table='user'").Count(&logs).Error)
	require.Equal(t, int64(1), logs)

	postRouter := f.router(2, false, http.MethodPost, func(c *gin.Context) {
		require.NoError(t, requesttx.DB(c.Request.Context()).Exec("UPDATE `"+f.prefix+"user` SET username='root-inherited' WHERE id=11").Error)
		stage(c, 1, "ok")
	})
	rec = f.request(t, postRouter, http.MethodPost, "/admin/auth.Admin/edit", `{"id":11,"username":"root-inherited"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, f.db.Table(q).Where("id=11 AND username='root-inherited'").Count(&logs).Error)
	require.Equal(t, int64(1), logs)
	var sensitiveLogs int64
	require.NoError(t, f.db.Table(f.table("security_sensitive_data_log")).Count(&sensitiveLogs).Error)
	require.Equal(t, int64(1), sensitiveLogs)
	var audit struct {
		AdminID       int32
		TargetAdminID int32
		IsCommitted   int32
		Before        string
		After         string
	}
	require.NoError(t, f.db.Raw("SELECT admin_id, target_admin_id, is_committed, `before`, `after` FROM "+f.table("security_sensitive_data_log")+" WHERE data_table=? AND id_value=? LIMIT 1", "user", 11).Scan(&audit).Error)
	require.Equal(t, int32(2), audit.AdminID)
	require.Equal(t, int32(4), audit.TargetAdminID)
	require.Equal(t, int32(1), audit.IsCommitted)
	require.Equal(t, "old-child", audit.Before)
	require.Equal(t, "root-inherited", audit.After)

	var recycleLogsBeforeSibling int64
	require.NoError(t, f.db.Table(f.table("security_data_recycle_log")).Count(&recycleLogsBeforeSibling).Error)
	siblingRouter := f.router(2, false, http.MethodDelete, func(c *gin.Context) {
		deleteRequested(t, c, f.prefix)
		stage(c, 1, "ok")
	})
	rec = f.request(t, siblingRouter, http.MethodDelete, "/admin/auth.Admin/del?ids[]=12", "")
	require.Equal(t, http.StatusForbidden, rec.Code)
	var recycleLogsAfterSibling int64
	require.NoError(t, f.db.Table(f.table("security_data_recycle_log")).Count(&recycleLogsAfterSibling).Error)
	require.Equal(t, recycleLogsBeforeSibling, recycleLogsAfterSibling)
	require.NoError(t, f.db.Table(f.table("security_sensitive_data_log")).Count(&sensitiveLogs).Error)
	require.Equal(t, int64(1), sensitiveLogs)
}

func TestSecurityMySQLDeleteCommitRollbackAndRestore(t *testing.T) {
	f := newSecurityFixture(t)
	q := f.table("user")
	deleteHandler := func(c *gin.Context) {
		deleteRequested(t, c, f.prefix)
		stage(c, 1, "ok")
	}
	r := f.router(2, false, http.MethodDelete, deleteHandler)
	rec := f.request(t, r, http.MethodDelete, "/admin/auth.Admin/del?ids[]=10", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var logID int32
	require.NoError(t, f.db.Table(f.table("security_data_recycle_log")).Select("id").Where("is_committed=1").Scan(&logID).Error)
	require.NotZero(t, logID)
	var rowCount int64
	f.db.Table(q).Where("id=10").Count(&rowCount)
	require.Zero(t, rowCount)
	recycleModel := model.NewDataRecycleLogModel(f.db, f.config, data_scope.NewClosureEnforcer(f.config))
	require.NoError(t, recycleModel.Restore(f.actorContext(2, false), []int32{logID}))
	f.db.Table(q).Where("id=10").Count(&rowCount)
	require.Equal(t, int64(1), rowCount)

	// A business error keeps the original response but rolls back both writes.
	f.db.Exec("DELETE FROM " + f.table("security_data_recycle_log"))
	r = f.router(2, false, http.MethodDelete, func(c *gin.Context) {
		require.NoError(t, requesttx.DB(c.Request.Context()).Exec("DELETE FROM `"+f.prefix+"user` WHERE id=10").Error)
		stage(c, 0, "business failed")
	})
	rec = f.request(t, r, http.MethodDelete, "/admin/auth.Admin/del?ids[]=10", "")
	require.Equal(t, http.StatusOK, rec.Code)
	f.db.Table(q).Where("id=10").Count(&rowCount)
	require.Equal(t, int64(1), rowCount)
	var logs int64
	f.db.Table(f.table("security_data_recycle_log")).Count(&logs)
	require.Zero(t, logs)
}

func TestSecurityMySQLPostOutcomesAndFailures(t *testing.T) {
	f := newSecurityFixture(t)
	q := f.table("user")
	post := func(handler gin.HandlerFunc) *httptest.ResponseRecorder {
		r := f.router(2, false, http.MethodPost, handler)
		return f.request(t, r, http.MethodPost, "/admin/auth.Admin/edit", `{"id":10,"username":"new","value":9}`)
	}
	rec := post(func(c *gin.Context) {
		require.NoError(t, requesttx.DB(c.Request.Context()).Exec("UPDATE `"+f.prefix+"user` SET username='new', value=9 WHERE id=10").Error)
		stage(c, 1, "ok")
	})
	require.Equal(t, http.StatusOK, rec.Code)
	var after, before string
	f.db.Table(f.table("security_sensitive_data_log")).Select("`before`, `after`").Order("id desc").Row().Scan(&before, &after)
	require.Equal(t, "old", before)
	require.Equal(t, "new", after)
	var logs int64
	f.db.Table(f.table("security_sensitive_data_log")).Count(&logs)
	require.Equal(t, int64(1), logs)
	post(func(c *gin.Context) {
		require.NoError(t, requesttx.DB(c.Request.Context()).Exec("UPDATE `"+f.prefix+"user` SET username='failed' WHERE id=10").Error)
		stage(c, 0, "business failed")
	})
	f.db.Table(q).Where("id=10 AND username='new'").Count(&logs)
	require.Equal(t, int64(1), logs)
	post(func(c *gin.Context) {
		require.NoError(t, requesttx.DB(c.Request.Context()).Exec("UPDATE `"+f.prefix+"user` SET username='new' WHERE id=10").Error)
		stage(c, 1, "ok")
	})
	f.db.Table(f.table("security_sensitive_data_log")).Count(&logs)
	require.Equal(t, int64(1), logs)
	// A log trigger failure aborts the request transaction and the business write.
	require.NoError(t, f.db.Exec("CREATE TRIGGER `"+f.prefix+"fail_sensitive_log` BEFORE INSERT ON `"+f.prefix+"security_sensitive_data_log` FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='audit failure'").Error)
	rec = post(func(c *gin.Context) {
		require.NoError(t, requesttx.DB(c.Request.Context()).Exec("UPDATE `"+f.prefix+"user` SET username='trigger-fail' WHERE id=10").Error)
		stage(c, 1, "ok")
	})
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	f.db.Table(q).Where("id=10 AND username='new'").Count(&logs)
	require.Equal(t, int64(1), logs)
}

func TestSecurityMySQLFailClosedWriterAndPanic(t *testing.T) {
	f := newSecurityFixture(t)
	q := f.table("user")
	for _, tc := range []struct {
		name    string
		handler gin.HandlerFunc
	}{
		{"missing outcome", func(c *gin.Context) {
			require.NoError(t, requesttx.DB(c.Request.Context()).Exec("UPDATE `"+f.prefix+"user` SET username='x' WHERE id=10").Error)
		}},
		{"direct writer", func(c *gin.Context) {
			require.NoError(t, requesttx.DB(c.Request.Context()).Exec("UPDATE `"+f.prefix+"user` SET username='x' WHERE id=10").Error)
			c.Header("Content-Type", "text/plain")
			c.Header("Set-Cookie", "sid=direct")
			c.String(200, "direct")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := f.router(2, false, http.MethodPost, tc.handler)
			rec := f.request(t, r, http.MethodPost, "/admin/auth.Admin/edit", `{"id":10,"username":"x"}`)
			require.NotEqual(t, http.StatusCreated, rec.Code)
			if tc.name == "direct writer" {
				require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
				require.Empty(t, rec.Header().Get("Set-Cookie"))
			}
			var got string
			f.db.Table(q).Select("username").Where("id=10").Scan(&got)
			require.Equal(t, "old", got)
		})
	}
	r := f.router(2, false, http.MethodPost, func(c *gin.Context) {
		require.NoError(t, requesttx.DB(c.Request.Context()).Exec("UPDATE `"+f.prefix+"user` SET username='panic' WHERE id=10").Error)
		stage(c, 1, "ok")
		panic("boom")
	})
	require.Panics(t, func() { f.request(t, r, http.MethodPost, "/admin/auth.Admin/edit", `{"id":10,"username":"panic"}`) })
	var got string
	f.db.Table(q).Select("username").Where("id=10").Scan(&got)
	require.Equal(t, "old", got)
}

func TestSecurityMySQLUnregisteredRouteFailsClosed(t *testing.T) {
	f := newSecurityFixture(t)
	require.NoError(t, f.db.Exec("INSERT INTO "+f.table("security_sensitive_data")+" (admin_id,name,controller,controller_as,data_table,primary_key,data_fields,status) VALUES (1,'unknown','unknown','unknown/action','user','id','{\"username\":\"username\"}','1')").Error)
	r := gin.New()
	s := NewSecurity(f.config, zap.NewNop(), f.db, data_scope.NewClosureEnforcer(f.config))
	r.Use(func(c *gin.Context) { _ = data_scope.SetActor(c, data_scope.Actor{AdminID: 2}) }, s.Handler())
	r.POST("/admin/unknown.Action/edit", func(c *gin.Context) { stage(c, 1, "bad") })
	rec := f.request(t, r, http.MethodPost, "/admin/unknown.Action/edit", `{"id":10}`)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
