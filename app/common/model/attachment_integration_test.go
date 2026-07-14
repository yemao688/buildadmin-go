package model

import (
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type attachmentRuntimeRow struct {
	ID             int32
	AdminID        int32
	UserID         int32
	URL            string
	Topic          string
	Width          int32
	Height         int32
	Name           string
	Size           int32
	Mimetype       string
	Quote          int32
	Storage        string
	Sha1           string
	CreateTime     int64
	LastUploadTime int64
}

func TestAttachmentModelMySQLDataScope(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	prefix := "att_rt_"
	closure, attachment, admins, users := prefix+"admin_closure", prefix+"attachment", "admins", "users"
	quote := func(s string) string { return "`" + s + "`" }
	tables := []string{closure, attachment, admins, users}
	for _, table := range tables {
		db.Exec("DROP TABLE IF EXISTS " + quote(table))
	}
	t.Cleanup(func() {
		for _, table := range tables {
			db.Exec("DROP TABLE IF EXISTS " + quote(table))
		}
		sqlDB.Close()
	})
	require.NoError(t, db.Exec("CREATE TABLE "+quote(closure)+" (ancestor_id INT NOT NULL, descendant_id INT NOT NULL, depth INT NOT NULL, PRIMARY KEY (ancestor_id, descendant_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+quote(attachment)+" (id INT PRIMARY KEY, topic VARCHAR(20) NOT NULL, admin_id INT NOT NULL, user_id INT NOT NULL, url VARCHAR(255) NOT NULL, width INT NOT NULL, height INT NOT NULL, name VARCHAR(120) NOT NULL, size INT NOT NULL, mimetype VARCHAR(100) NOT NULL, quote INT NOT NULL, storage VARCHAR(50) NOT NULL, sha1 VARCHAR(40) NOT NULL, create_time BIGINT NOT NULL, last_upload_time BIGINT NOT NULL, KEY idx_admin_id (admin_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+quote(admins)+" (id INT PRIMARY KEY, username VARCHAR(20) NOT NULL, nickname VARCHAR(20) NOT NULL)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+quote(users)+" (id INT PRIMARY KEY, username VARCHAR(20) NOT NULL, nickname VARCHAR(20) NOT NULL)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+quote(closure)+" VALUES (1,1,0),(1,2,1),(1,3,1),(1,4,2),(2,2,0),(2,4,1),(3,3,0),(4,4,0)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+quote(admins)+" VALUES (1,'a','A'),(2,'b','B'),(3,'c','C'),(4,'d','D')").Error)
	require.NoError(t, db.Exec("INSERT INTO "+quote(users)+" VALUES (1,'u1','U1'),(2,'u2','U2'),(3,'u3','U3'),(4,'u4','U4')").Error)
	require.NoError(t, db.Exec("INSERT INTO "+quote(attachment)+" (id,topic,admin_id,user_id,url,width,height,name,size,mimetype,quote,storage,sha1,create_time,last_upload_time) VALUES (1,'a',1,1,'/a',1,1,'A',1,'x',0,'local','a',1,1),(2,'b',2,2,'/b',1,1,'B',1,'x',0,'local','b',1,1),(3,'c',3,3,'/c',1,1,'C',1,'x',0,'local','c',1,1),(4,'d',4,4,'/d',1,1,'D',1,'x',0,'local','d',1,1)").Error)

	e := data_scope.NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: prefix}})
	m := NewAttachmentModel(db, &conf.Configuration{Database: conf.Database{Prefix: prefix}}, e)
	ctx := func(id int32, unrestricted bool) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/?limit=100&page=1&order=id,asc", nil)
		c.Set(data_scope.ActorContextKey, data_scope.Actor{AdminID: id, Unrestricted: unrestricted})
		return c
	}
	labels := func(c *gin.Context) []string {
		rows, total, err := m.List(c)
		require.NoError(t, err)
		var count int64
		scoped := m.scoped(c, db.Table(attachment+" AS attachment"))
		require.NoError(t, scoped.Count(&count).Error)
		require.Equal(t, total, count)
		got := make([]string, len(rows))
		for i, row := range rows {
			got[i] = row.Name
		}
		return got
	}
	require.Equal(t, []string{"B", "D"}, labels(ctx(2, false)))
	require.Equal(t, []string{"A", "B", "C", "D"}, labels(ctx(1, false)))
	require.Equal(t, []string{"C"}, labels(ctx(3, false)))
	_, err = m.GetOne(ctx(2, false), 3)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)

	edit := Attachment{ID: 2, Topic: "", URL: "/edited", Width: 0, Height: 0, Name: "", Size: 0, Mimetype: "", Quote: 0, Storage: "", Sha1: ""}
	require.NoError(t, m.Edit(ctx(2, false), edit))
	var raw attachmentRuntimeRow
	require.NoError(t, db.Table(attachment).Where("id=2").Take(&raw).Error)
	require.Equal(t, int32(2), raw.AdminID)
	require.Equal(t, int32(2), raw.UserID)
	require.Equal(t, "/edited", raw.URL)
	require.Equal(t, "", raw.Name)
	before := raw
	require.ErrorIs(t, m.Edit(ctx(3, false), Attachment{ID: 2, Name: "hijack"}), gorm.ErrRecordNotFound)
	require.NoError(t, db.Table(attachment).Where("id=2").Take(&raw).Error)
	require.Equal(t, before, raw)

	// A mixed-owner bulk delete must be all-or-nothing.
	require.Error(t, m.Del(ctx(2, false), []int32{2, 3}))
	require.Equal(t, int64(4), countRows(t, db, attachment))
	require.NoError(t, m.Del(ctx(2, false), []int32{2, 4}))
	var remaining int64
	require.NoError(t, db.Table(attachment).Count(&remaining).Error)
	require.Equal(t, int64(2), remaining)
	require.Error(t, m.Del(ctx(3, false), []int32{1}))
	require.Equal(t, int64(2), countRows(t, db, attachment))
	require.Equal(t, []string{}, labels(ctx(2, false)))
	require.Equal(t, []string{"A", "C"}, labels(ctx(1, true)))
	_, _, err = m.List(ctx(0, false))
	require.Error(t, err)
	_, err = m.GetOne(ctx(0, false), 1)
	require.Error(t, err)
	require.Error(t, m.Edit(ctx(0, false), Attachment{ID: 1, Name: "x"}))
	require.Error(t, m.Del(ctx(0, false), []int32{1}))
}

func countRows(t *testing.T, db *gorm.DB, table string) int64 {
	var n int64
	require.NoError(t, db.Table(table).Count(&n).Error)
	return n
}
