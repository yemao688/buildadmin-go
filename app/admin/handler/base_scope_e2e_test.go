package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type scopeE2EModel struct {
	db       *gorm.DB
	table    string
	policy   data_scope.ResourcePolicy
	enforcer data_scope.Enforcer
}

func (m *scopeE2EModel) DB() *gorm.DB                   { return m.db }
func (m *scopeE2EModel) Table() string                  { return m.table }
func (m *scopeE2EModel) DBFor(context.Context) *gorm.DB { return m.db }
func (m *scopeE2EModel) Transaction(_ context.Context, fn func(*gorm.DB) error) error {
	return m.db.Transaction(fn)
}
func (m *scopeE2EModel) ScopeDB(ctx *gin.Context, db *gorm.DB) *gorm.DB {
	if m.policy.Mode == data_scope.ModeNone {
		return db
	}
	return m.enforcer.Scope(ctx, db, data_scope.OwnerRef{TableAlias: m.table, Column: m.policy.OwnerColumn})
}

func TestBaseScopedCRUDMySQL(t *testing.T) {
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("BUILDADMIN_TEST_MYSQL_DSN must point to disposable MySQL 8.4")
	}

	db, err := gorm.Open(mysql.Open(dsn))
	require.NoError(t, err)
	prefix := fmt.Sprintf("ba_base_scope_%d_", os.Getpid())
	db.Config.NamingStrategy = schema.NamingStrategy{TablePrefix: prefix}
	items := prefix + "items"
	closure := prefix + "admin_closure"
	for _, table := range []string{items, closure} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS `"+table+"`").Error)
		table := table
		t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error })
	}
	require.NoError(t, db.Exec("CREATE TABLE `"+closure+"` (ancestor_id BIGINT NOT NULL, descendant_id BIGINT NOT NULL, PRIMARY KEY (ancestor_id, descendant_id))").Error)
	require.NoError(t, db.Exec("CREATE TABLE `"+items+"` (id INT PRIMARY KEY, admin_id INT NOT NULL, status TINYINT NOT NULL, weigh INT NOT NULL, name VARCHAR(100) NOT NULL)").Error)
	for _, pair := range [][2]int{{1, 1}, {2, 2}} {
		require.NoError(t, db.Exec("INSERT INTO `"+closure+"` (ancestor_id, descendant_id) VALUES (?, ?)", pair[0], pair[1]).Error)
	}
	for _, row := range [][4]interface{}{{1, 1, 1, 2}, {2, 1, 1, 1}, {3, 2, 1, 2}, {4, 2, 1, 1}} {
		require.NoError(t, db.Exec("INSERT INTO `"+items+"` (id, admin_id, status, weigh, name) VALUES (?, ?, ?, ?, ?)", row[0], row[1], row[2], row[3], fmt.Sprintf("item-%v", row[0])).Error)
	}

	restricted := newScopeE2EModel(db, items, data_scope.ModeAuto, prefix)
	global := newScopeE2EModel(db, items, data_scope.ModeNone, prefix)
	actor := func() *gin.Context {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		require.NoError(t, data_scope.SetActor(ctx, data_scope.Actor{AdminID: 1}))
		return ctx
	}

	h := &Base{currentM: restricted}
	ctx := actor()
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/items/edit?id=1", nil)
	h.One(ctx)
	require.Equal(t, http.StatusOK, ctx.Writer.Status())

	ctx = actor()
	ctx.Request = httptest.NewRequest(http.MethodGet, "/admin/items/edit?id=3", nil)
	h.One(ctx)
	require.NotEqual(t, http.StatusOK, ctx.Writer.Status())

	ctx = actor()
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/items/edit", jsonBody(`{"id":1,"status":0}`))
	h.MaybePartialEdit(ctx, map[string]bool{"status": true})
	require.Equal(t, http.StatusOK, ctx.Writer.Status())

	ctx = actor()
	ctx.Request = httptest.NewRequest(http.MethodPost, "/admin/items/edit", jsonBody(`{"id":3,"status":0}`))
	h.MaybePartialEdit(ctx, map[string]bool{"status": true})
	require.NotEqual(t, http.StatusOK, ctx.Writer.Status())
	var status int
	require.NoError(t, db.Raw("SELECT status FROM `"+items+"` WHERE id=3").Scan(&status).Error)
	require.Equal(t, 1, status)

	ctx = actor()
	require.NoError(t, Sortable(ctx, restricted, 2, 1, "up", "weigh,desc"))

	var before3, before4 int
	require.NoError(t, db.Raw("SELECT weigh FROM `"+items+"` WHERE id=3").Scan(&before3).Error)
	require.NoError(t, db.Raw("SELECT weigh FROM `"+items+"` WHERE id=4").Scan(&before4).Error)
	require.Error(t, Sortable(actor(), restricted, 1, 3, "down", "weigh,desc"))
	var after3, after4 int
	require.NoError(t, db.Raw("SELECT weigh FROM `"+items+"` WHERE id=3").Scan(&after3).Error)
	require.NoError(t, db.Raw("SELECT weigh FROM `"+items+"` WHERE id=4").Scan(&after4).Error)
	require.Equal(t, before3, after3)
	require.Equal(t, before4, after4)

	missingActor, _ := gin.CreateTestContext(httptest.NewRecorder())
	globalBase := &Base{currentM: global}
	missingActor.Request = httptest.NewRequest(http.MethodGet, "/admin/items/edit?id=3", nil)
	globalBase.One(missingActor)
	require.Equal(t, http.StatusOK, missingActor.Writer.Status())
	globalSwitch, _ := gin.CreateTestContext(httptest.NewRecorder())
	globalSwitch.Request = httptest.NewRequest(http.MethodPost, "/admin/items/edit", jsonBody(`{"id":3,"status":0}`))
	require.True(t, globalBase.MaybePartialEdit(globalSwitch, map[string]bool{"status": true}))
	require.Equal(t, http.StatusOK, globalSwitch.Writer.Status())
	globalSort, _ := gin.CreateTestContext(httptest.NewRecorder())
	require.NoError(t, Sortable(globalSort, global, 4, 3, "up", "weigh,desc"))
}

func newScopeE2EModel(db *gorm.DB, table string, mode data_scope.Mode, prefix string) *scopeE2EModel {
	cfg := &data_scope.Config{Mode: mode, OwnerColumn: "admin_id"}
	return &scopeE2EModel{db: db, table: table, policy: data_scope.ResourcePolicy{Mode: mode, OwnerColumn: cfg.OwnerColumn}, enforcer: data_scope.NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: prefix}})}
}

func jsonBody(body string) *strings.Reader { return strings.NewReader(body) }
