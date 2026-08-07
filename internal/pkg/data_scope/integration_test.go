package data_scope

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"buildadmin-go/internal/pkg/testutil"
	"buildadmin-go/internal/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	db, _ := testutil.OpenMySQL(t)
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

// captureLogger records every SQL statement executed on a connection. It is
// used to prove that a cached request drops the per-query self-EXISTS
// subquery: statements carrying the self_closure alias are the only ones the
// one-time verification is meant to eliminate.
type captureLogger struct {
	logger.Interface
	mu   sync.Mutex
	sqls []string
}

func (l *captureLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	l.mu.Lock()
	l.sqls = append(l.sqls, sql)
	l.mu.Unlock()
	l.Interface.Trace(ctx, begin, fc, err)
}

func (l *captureLogger) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sqls = nil
}

// selfClosureStatements counts executed statements whose WHERE carries the
// self-EXISTS subquery (AS self_closure alias).
func (l *captureLogger) selfClosureStatements() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, s := range l.sqls {
		if strings.Contains(s, "self_closure") {
			n++
		}
	}
	return n
}

// closureStatements counts executed statements referencing the closure table
// at all (one-time verification plus the branch EXISTS of every scoped query).
func (l *captureLogger) closureStatements() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, s := range l.sqls {
		if strings.Contains(s, "admin_closure") {
			n++
		}
	}
	return n
}

// TestClosureEnforcerSelfRowCacheMySQL verifies H8 behavior end to end:
//   - without the per-request marker, every scoped construction emits a
//     statement touching admin_closure (self-EXISTS + branch EXISTS);
//   - with the marker set (as the login middleware does), the one-time
//     verification is the only admin_closure statement and the scoped rows
//     are identical to the uncached baseline;
//   - a verified-missing self-row keeps the full condition and denies scope
//     exactly like today (zero rows).
func TestClosureEnforcerSelfRowCacheMySQL(t *testing.T) {
	db, cfg := testutil.OpenMySQL(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	prefix := fmt.Sprintf("ds_h8_%d_", os.Getpid())
	closureTable := prefix + "admin_closure"
	itemsTable := prefix + "items"
	q := func(name string) string { return "`" + name + "`" }
	for _, table := range []string{closureTable, itemsTable} {
		db.Exec("DROP TABLE IF EXISTS " + q(table))
	}
	t.Cleanup(func() {
		for _, table := range []string{closureTable, itemsTable} {
			db.Exec("DROP TABLE IF EXISTS " + q(table))
		}
	})

	require.NoError(t, db.Exec("CREATE TABLE "+q(closureTable)+" (ancestor_id INT NOT NULL, descendant_id INT NOT NULL, PRIMARY KEY (ancestor_id, descendant_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE "+q(itemsTable)+" (id INT PRIMARY KEY, admin_id INT NOT NULL, label VARCHAR(32) NOT NULL)").Error)
	// Admin 1 is the root; admin 2 has a self-row and owns items 2,3; admin 99
	// deliberately has no closure row at all.
	require.NoError(t, db.Exec("INSERT INTO "+q(closureTable)+" (ancestor_id, descendant_id) VALUES (1,1),(1,2),(1,3),(2,2),(2,3),(3,3)").Error)
	require.NoError(t, db.Exec("INSERT INTO "+q(itemsTable)+" (id, admin_id, label) VALUES (1,1,'A'),(2,2,'B'),(3,3,'C')").Error)

	// A second connection with a capturing logger counts admin_closure hits.
	capture := &captureLogger{Interface: logger.Default.LogMode(logger.Silent)}
	captured, err := gorm.Open(mysql.Open(testutil.MySQLDSN(cfg.MysqlTest, cfg.Database.Database)), &gorm.Config{Logger: capture})
	require.NoError(t, err)
	t.Cleanup(func() {
		if c, cerr := captured.DB(); cerr == nil {
			_ = c.Close()
		}
	})

	enforcer := NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: prefix}})
	items := q(itemsTable) + " AS i"
	actorCtx := func(id int32) *gin.Context {
		c, _ := gin.CreateTestContext(nil)
		c.Set(actorContextKey, Actor{AdminID: id})
		return c
	}
	// verifySelfRow mirrors the login middleware's one-time verification
	// (AuthRepository.HasClosureSelfRow executes this exact query shape).
	verifySelfRow := func(c *gin.Context, id int32) {
		var count int64
		require.NoError(t, captured.Table(q(closureTable)).Where("ancestor_id = ? AND descendant_id = ?", id, id).Count(&count).Error)
		MarkSelfRowChecked(c, count > 0)
	}

	// Baseline without the marker: both scoped constructions (count + find)
	// carry the self-EXISTS subquery.
	var baselineCount int64
	require.NoError(t, enforcer.Scope(actorCtx(2), captured.Table(items), OwnerRef{TableAlias: "i", Column: "admin_id"}).Count(&baselineCount).Error)
	require.Equal(t, int64(2), baselineCount)
	var baselineRows []runtimeScopeItem
	require.NoError(t, enforcer.Scope(actorCtx(2), captured.Table(items), OwnerRef{TableAlias: "i", Column: "admin_id"}).Order("i.id").Find(&baselineRows).Error)
	require.Len(t, baselineRows, 2)
	require.Equal(t, 2, capture.selfClosureStatements(), "uncached request: each scoped construction carries the self-EXISTS")

	// Cached request: one verification + two scoped constructions. The scoped
	// statements must drop the self-EXISTS; the verification is the only
	// closure statement that is not a branch-EXISTS.
	capture.reset()
	cachedCtx, _ := gin.CreateTestContext(nil)
	cachedCtx.Set(actorContextKey, Actor{AdminID: 2})
	verifySelfRow(cachedCtx, 2)
	var count int64
	require.NoError(t, enforcer.Scope(cachedCtx, captured.Table(items), OwnerRef{TableAlias: "i", Column: "admin_id"}).Count(&count).Error)
	require.Equal(t, int64(2), count)
	var rows []runtimeScopeItem
	require.NoError(t, enforcer.Scope(cachedCtx, captured.Table(items), OwnerRef{TableAlias: "i", Column: "admin_id"}).Order("i.id").Find(&rows).Error)
	require.Equal(t, baselineRows, rows, "cached scope must return exactly the uncached row set")
	require.Equal(t, 0, capture.selfClosureStatements(), "cached request: scoped statements must drop the self-EXISTS")
	require.Equal(t, 3, capture.closureStatements(), "cached request: one verification + two branch-EXISTS statements")

	// Verified missing self-row: the full condition is kept and denies scope
	// exactly like today (zero rows).
	capture.reset()
	missingCtx, _ := gin.CreateTestContext(nil)
	missingCtx.Set(actorContextKey, Actor{AdminID: 99})
	verifySelfRow(missingCtx, 99)
	var none []runtimeScopeItem
	require.NoError(t, enforcer.Scope(missingCtx, captured.Table(items), OwnerRef{TableAlias: "i", Column: "admin_id"}).Find(&none).Error)
	require.Empty(t, none, "actor without a closure self-row must see zero rows")
	require.Equal(t, 1, capture.selfClosureStatements(), "verified-missing self-row keeps the self-EXISTS guard")

	// Unrestricted actors bypass scope regardless of the marker.
	unrestrictedCtx, _ := gin.CreateTestContext(nil)
	unrestrictedCtx.Set(actorContextKey, Actor{AdminID: 2, Unrestricted: true})
	MarkSelfRowChecked(unrestrictedCtx, false)
	var all []runtimeScopeItem
	require.NoError(t, enforcer.Scope(unrestrictedCtx, captured.Table(items), OwnerRef{TableAlias: "i", Column: "admin_id"}).Order("i.id").Find(&all).Error)
	require.Len(t, all, 3)
}
