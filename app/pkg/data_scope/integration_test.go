package data_scope

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type runtimeScopeItem struct {
	ID      int32  `gorm:"column:id"`
	AdminID int32  `gorm:"column:admin_id"`
	Label   string `gorm:"column:label"`
}

type runtimeScopeAdmin struct {
	ID int32 `gorm:"column:id"`
}

func TestClosureEnforcerMySQL(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("BUILDADMIN_TEST_MYSQL_DSN is not set")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	prefix := fmt.Sprintf("ds_rt_%d_", os.Getpid())
	closureTable := prefix + "admin_closure"
	itemsTable := prefix + "items"
	adminsTable := prefix + "admins"
	q := func(name string) string { return "`" + name + "`" }
	for _, table := range []string{closureTable, itemsTable, adminsTable} {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
	}
	t.Cleanup(func() {
		for _, table := range []string{closureTable, itemsTable, adminsTable} {
			db.Exec("DROP TABLE IF EXISTS " + q(table))
		}
		sqlDB.Close()
	})

	require.NoError(t, db.Exec("CREATE TABLE "+q(closureTable)+" (ancestor_id INT NOT NULL, descendant_id INT NOT NULL, PRIMARY KEY (ancestor_id, descendant_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(itemsTable)+" (id INT PRIMARY KEY, admin_id INT NOT NULL, label VARCHAR(32) NOT NULL)").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(adminsTable)+" (id INT PRIMARY KEY)").Error)
	closureRows := []string{"(1,1)", "(1,2)", "(1,3)", "(1,4)", "(2,2)", "(2,4)", "(3,3)", "(4,4)"}
	require.NoError(t, db.Exec("INSERT INTO "+q(closureTable)+" (ancestor_id, descendant_id) VALUES "+strings.Join(closureRows, ",")).Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(itemsTable)+" (id, admin_id, label) VALUES (1,1,'A'),(2,2,'B'),(3,3,'C'),(4,4,'D')").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(adminsTable)+" (id) VALUES (1),(2),(3),(4)").Error)

	enforcer := NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: prefix}})
	ctx := func(id int32, unrestricted bool) *gin.Context {
		c, _ := gin.CreateTestContext(nil)
		c.Set(actorContextKey, Actor{AdminID: id, Unrestricted: unrestricted})
		return c
	}
	listItems := func(id int32) []runtimeScopeItem {
		var got []runtimeScopeItem
		scoped := enforcer.Scope(ctx(id, false), db.Table(q(itemsTable)+" AS i"), OwnerRef{TableAlias: "i", Column: "admin_id"})
		require.NoError(t, scoped.Order("i.id").Find(&got).Error)
		return got
	}
	requireLabels := func(want []string, got []runtimeScopeItem) {
		labels := make([]string, len(got))
		for i := range got {
			labels[i] = got[i].Label
		}
		require.Equal(t, want, labels)
	}
	requireLabels([]string{"A", "B", "C", "D"}, listItems(1))
	requireLabels([]string{"B", "D"}, listItems(2))
	requireLabels([]string{"C"}, listItems(3))

	var count int64
	scopedB := enforcer.Scope(ctx(2, false), db.Table(q(itemsTable)+" AS i"), OwnerRef{TableAlias: "i", Column: "admin_id"})
	require.NoError(t, scopedB.Count(&count).Error)
	require.Equal(t, int64(2), count)
	var bRows []runtimeScopeItem
	require.NoError(t, scopedB.Order("i.id").Find(&bRows).Error)
	require.Len(t, bRows, int(2))

	require.NoError(t, db.Exec("DELETE FROM "+q(closureTable)+" WHERE ancestor_id=2 AND descendant_id=2").Error)
	requireLabels([]string{}, listItems(2))
	require.NoError(t, db.Exec("INSERT INTO "+q(closureTable)+" (ancestor_id, descendant_id) VALUES (2,2)").Error)
	requireLabels([]string{"B", "D"}, listItems(2))

	var all []runtimeScopeItem
	require.NoError(t, enforcer.Scope(ctx(99, true), db.Table(q(itemsTable)+" AS i"), OwnerRef{TableAlias: "i", Column: "admin_id"}).Find(&all).Error)
	require.Len(t, all, 4)
	var ownAdmin []runtimeScopeAdmin
	require.NoError(t, enforcer.Scope(ctx(2, false), db.Table(q(adminsTable)+" AS a"), OwnerRef{TableAlias: "a", Column: "id"}).Order("a.id").Find(&ownAdmin).Error)
	require.Equal(t, []runtimeScopeAdmin{{ID: 2}, {ID: 4}}, ownAdmin)

	var joined []runtimeScopeItem
	require.NoError(t, enforcer.Scope(ctx(2, false), db.Table(q(itemsTable)+" AS i").Joins("JOIN "+q(adminsTable)+" AS a ON a.id = i.admin_id"), OwnerRef{TableAlias: "i", Column: "admin_id"}).Select("i.id, i.admin_id, i.label").Find(&joined).Error)
	require.Len(t, joined, 2)

	require.NoError(t, db.Exec("DROP TABLE "+q(closureTable)).Error)
	var missing []runtimeScopeItem
	err = enforcer.Scope(ctx(2, false), db.Table(q(itemsTable)+" AS i"), OwnerRef{TableAlias: "i", Column: "admin_id"}).Find(&missing).Error
	require.Error(t, err)
	require.NotContains(t, strings.ToLower(err.Error()), "where")
}
