package model

import (
	"errors"
	"net/http/httptest"
	"os"
	"testing"

	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/app/pkg/requesttx"
	"go-build-admin/conf"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// testDialector is a minimal in-memory dialector for unit tests.
// It is duplicated from app/pkg/data_scope so that model tests do not
// need a running MySQL instance.
type testDialector struct{}

func (testDialector) Name() string                                   { return "test" }
func (testDialector) Initialize(*gorm.DB) error                      { return nil }
func (testDialector) Migrator(*gorm.DB) gorm.Migrator                { return nil }
func (testDialector) DataTypeOf(*schema.Field) string                { return "" }
func (testDialector) DefaultValueOf(*schema.Field) clause.Expression { return nil }
func (testDialector) BindVarTo(w clause.Writer, stmt *gorm.Statement, v interface{}) {
	_ = stmt
	_ = v
	w.WriteByte('?')
}
func (testDialector) QuoteTo(w clause.Writer, s string)              { w.WriteString("`" + s + "`") }
func (testDialector) Explain(sql string, vars ...interface{}) string { return sql }

func openAdminTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(testDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	return db
}

func openMySQLAdminTestDB(t *testing.T, prefix string) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("BUILDADMIN_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set BUILDADMIN_TEST_MYSQL_DSN to run MySQL integration tests")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
			TablePrefix:   prefix,
		},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}

func adminTestContext(t *testing.T, unrestricted bool) *gin.Context {
	return adminTestContextID(t, 5, unrestricted)
}

func adminTestContextID(t *testing.T, id int32, unrestricted bool) *gin.Context {
	t.Helper()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/admin/auth.Admin/edit", nil)
	var actor data_scope.Actor
	var err error
	if unrestricted {
		actor, err = data_scope.NewUnrestrictedActor(id)
	} else {
		actor, err = data_scope.NewActor(id)
	}
	if err != nil {
		t.Fatalf("actor: %v", err)
	}
	ctx.Set(data_scope.ActorContextKey, actor)
	return ctx
}

func TestAdminModelScopedFailClosed(t *testing.T) {
	db := openAdminTestDB(t)
	m := NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

	ctx, _ := gin.CreateTestContext(nil)
	scoped := m.scoped(ctx)(db.Model(&Admin{}))
	if !errors.Is(scoped.Error, data_scope.ErrScopedAccessDenied) {
		t.Fatalf("missing actor should fail closed, got %v", scoped.Error)
	}
}

func TestAdminModelScopedUnrestrictedBypass(t *testing.T) {
	db := openAdminTestDB(t)
	m := NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

	ctx := adminTestContext(t, true)
	scoped := m.scoped(ctx)(db.Model(&Admin{}))
	if scoped.Error != nil {
		t.Fatalf("unrestricted actor should receive clean DB, got %v", scoped.Error)
	}
}

func TestAdminModelActorRequiresRealRequest(t *testing.T) {
	db := openAdminTestDB(t)
	m := NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	if _, err := m.actor(nil); !errors.Is(err, data_scope.ErrScopedAccessDenied) {
		t.Fatalf("nil context should fail closed, got %v", err)
	}
	ctx, _ := gin.CreateTestContext(nil)
	if _, err := m.actor(ctx); !errors.Is(err, data_scope.ErrScopedAccessDenied) {
		t.Fatalf("nil request should fail closed, got %v", err)
	}
}

func prepareAdminModelMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	db := openMySQLAdminTestDB(t, "ba_")
	_ = db.Exec("DROP TRIGGER IF EXISTS ba_admin_closure_delete_block")
	_ = db.Migrator().DropTable(&AdminClosure{}, &AdminGroupAccess{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}, &AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)
	return db
}

func linkAdmins(t *testing.T, db *gorm.DB, links ...struct {
	node   Admin
	parent *int32
}) {
	t.Helper()
	h := newAdminHierarchy("ba_")
	ctx := adminTestContext(t, true)
	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, link := range links {
			if err := h.LinkNewNode(ctx.Request.Context(), tx, link.node.ID, link.parent); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("link admins: %v", err)
	}
}

func TestNormalizeAdminIDs(t *testing.T) {
	got, err := normalizeAdminIDs([]int32{4, 4, 2, 4})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 4 || got[1] != 2 {
		t.Fatalf("deduped IDs = %v", got)
	}
	for _, ids := range [][]int32{{0}, {-1}, {2, 0, 3}} {
		if _, err := normalizeAdminIDs(ids); err == nil {
			t.Fatalf("normalizeAdminIDs(%v) should reject non-positive IDs", ids)
		}
	}
}

func TestAdminAddActiveRequestTransactionRollsBackWithOuterFailure(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	m := NewAdminModel(db, hierarchyConfig("ba_"))
	ctx := adminTestContext(t, true)
	missingParent := int32(999999)
	admin := Admin{ParentID: &missingParent, Username: "requesttx-failure", Status: "enable"}

	err := db.Transaction(func(tx *gorm.DB) error {
		ctx.Request = ctx.Request.WithContext(requesttx.Bind(ctx.Request.Context(), tx))
		if err := m.Add(ctx, admin, nil); err == nil {
			t.Fatal("expected hierarchy validation failure")
		}
		return errors.New("outer business failure")
	})
	if err == nil {
		t.Fatal("expected outer transaction failure")
	}

	var count int64
	if err := db.Model(&Admin{}).Where("username = ?", admin.Username).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed request transaction leaked %d admin rows", count)
	}
}

func TestAdminAddWithoutRequestTransactionCommitsOwnTransaction(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	m := NewAdminModel(db, hierarchyConfig("ba_"))
	ctx := adminTestContext(t, true)
	admin := Admin{Username: "requesttx-success", Status: "enable"}

	if err := m.Add(ctx, admin, nil); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&Admin{}).Where("username = ?", admin.Username).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected committed admin row, got %d", count)
	}
}

func TestRestrictedDeleteVisibleLeafAndRejectInvisibleSibling(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	root := createAdminForHierarchy(t, db, "root")
	actor := createAdminForHierarchy(t, db, "actor")
	leaf := createAdminForHierarchy(t, db, "leaf")
	sibling := createAdminForHierarchy(t, db, "sibling")
	linkAdmins(t, db,
		struct {
			node   Admin
			parent *int32
		}{root, nil},
		struct {
			node   Admin
			parent *int32
		}{actor, &root.ID},
		struct {
			node   Admin
			parent *int32
		}{leaf, &actor.ID},
		struct {
			node   Admin
			parent *int32
		}{sibling, &root.ID},
	)

	m := NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	ctx := adminTestContextID(t, actor.ID, false)
	if err := m.Del(ctx, []int32{leaf.ID, leaf.ID}); err != nil {
		t.Fatalf("restricted leaf delete: %v", err)
	}
	var count int64
	db.Model(&Admin{}).Where("id = ?", leaf.ID).Count(&count)
	if count != 0 {
		t.Fatal("leaf admin row was not deleted")
	}
	db.Model(&AdminClosure{}).Where("descendant_id = ?", leaf.ID).Count(&count)
	if count != 0 {
		t.Fatal("leaf closure rows were not deleted")
	}
	if err := m.Del(ctx, []int32{sibling.ID}); err == nil {
		t.Fatal("restricted actor deleted an invisible sibling")
	}
	db.Model(&Admin{}).Where("id = ?", sibling.ID).Count(&count)
	if count != 1 {
		t.Fatal("invisible sibling was deleted")
	}
}

func TestDeleteClosureFailureRollsBackAdminAndClosure(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	actor := createAdminForHierarchy(t, db, "actor")
	leaf := createAdminForHierarchy(t, db, "leaf")
	linkAdmins(t, db,
		struct {
			node   Admin
			parent *int32
		}{actor, nil},
		struct {
			node   Admin
			parent *int32
		}{leaf, &actor.ID},
	)
	if err := db.Exec("CREATE TRIGGER ba_admin_closure_delete_block BEFORE DELETE ON ba_admin_closure FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'closure delete blocked'").Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	defer db.Exec("DROP TRIGGER IF EXISTS ba_admin_closure_delete_block")

	m := NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	if err := m.Del(adminTestContextID(t, actor.ID, false), []int32{leaf.ID}); err == nil {
		t.Fatal("expected closure deletion failure")
	}
	var count int64
	db.Model(&Admin{}).Where("id = ?", leaf.ID).Count(&count)
	if count != 1 {
		t.Fatal("admin delete was not rolled back")
	}
	db.Model(&AdminClosure{}).Where("descendant_id = ?", leaf.ID).Count(&count)
	if count == 0 {
		t.Fatal("closure delete was not rolled back")
	}
}

func TestRestrictedHierarchyWritersRejectRoot(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	actorAdmin := createAdminForHierarchy(t, db, "actor")
	candidate := createAdminForHierarchy(t, db, "candidate")
	linkAdmins(t, db, struct {
		node   Admin
		parent *int32
	}{actorAdmin, nil})
	ctx := adminTestContextID(t, actorAdmin.ID, false)
	actor, _ := data_scope.NewActor(actorAdmin.ID)
	enforcer := data_scope.NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	h := newAdminHierarchy("ba_")
	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.LinkNewNodeWithScope(ctx, tx, candidate.ID, nil, actor, enforcer)
	}); err == nil {
		t.Fatal("restricted LinkNewNodeWithScope root creation should fail")
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.MoveSubtreeWithScope(ctx, tx, actorAdmin.ID, nil, actor, enforcer)
	}); err == nil {
		t.Fatal("restricted MoveSubtreeWithScope root move should fail")
	}
}

func TestValidateOrMoveWithoutParentChangeChecksOnlyTargetAndClosure(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	invisibleParent := createAdminForHierarchy(t, db, "invisible-parent")
	actorAdmin := createAdminForHierarchy(t, db, "actor")
	linkAdmins(t, db, struct {
		node   Admin
		parent *int32
	}{invisibleParent, nil}, struct {
		node   Admin
		parent *int32
	}{actorAdmin, &invisibleParent.ID})
	ctx := adminTestContextID(t, actorAdmin.ID, false)
	actor, _ := data_scope.NewActor(actorAdmin.ID)
	enforcer := data_scope.NewClosureEnforcer(&conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	h := newAdminHierarchy("ba_")
	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.ValidateOrMoveWithScope(ctx, tx, actorAdmin.ID, false, actorAdmin.ParentID, actor, enforcer)
	}); err != nil {
		t.Fatalf("ordinary self edit should not require invisible ancestor scope: %v", err)
	}
	if err := db.Model(&Admin{}).Where("id = ?", actorAdmin.ID).Update("parent_id", nil).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.ValidateOrMoveWithScope(ctx, tx, actorAdmin.ID, false, nil, actor, enforcer)
	}); err == nil {
		t.Fatal("direct-parent/closure mismatch should fail")
	}
}

func TestSelectTreeExcludesDescendants(t *testing.T) {
	db := openMySQLAdminTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &AdminGroupAccess{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}, &AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)

	ctx := adminTestContext(t, true)
	m := NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	h := newAdminHierarchy("ba_")

	root := createAdminForHierarchy(t, db, "root")
	child := createAdminForHierarchy(t, db, "child") // name used only for matching
	grand := createAdminForHierarchy(t, db, "grand")

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := h.LinkNewNode(ctx.Request.Context(), tx, root.ID, nil); err != nil {
			return err
		}
		if err := h.LinkNewNode(ctx.Request.Context(), tx, child.ID, &root.ID); err != nil {
			return err
		}
		return h.LinkNewNode(ctx.Request.Context(), tx, grand.ID, &child.ID)
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Exclude the child node: both child and grandchild must disappear.
	list, err := m.SelectTree(ctx, child.ID, "")
	if err != nil {
		t.Fatalf("SelectTree: %v", err)
	}
	if len(list) != 1 || list[0].ID != root.ID {
		t.Fatalf("expected only root, got %+v", list)
	}
	matched, err := m.SelectTree(ctx, child.ID, "grand")
	if err != nil {
		t.Fatalf("keyword SelectTree: %v", err)
	}
	if len(matched) != 0 {
		t.Fatalf("excluded descendant leaked through keyword search: %+v", matched)
	}
}

func TestSwitchStatusScoped(t *testing.T) {
	db := openMySQLAdminTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &AdminGroupAccess{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}, &AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)

	m := NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
	h := newAdminHierarchy("ba_")

	// root (unrestricted) and a subordinate of actor 5.
	ctx := adminTestContext(t, true)
	root := createAdminForHierarchy(t, db, "root")
	actor5 := createAdminForHierarchy(t, db, "actor5")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := h.LinkNewNode(ctx.Request.Context(), tx, root.ID, nil); err != nil {
			return err
		}
		return h.LinkNewNode(ctx.Request.Context(), tx, actor5.ID, &root.ID)
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Actor 5 is restricted and can only see itself.
	actor5Ctx := adminTestContextID(t, actor5.ID, false)
	if err := m.SwitchStatus(actor5Ctx, root.ID, "disable"); err == nil {
		t.Fatal("restricted actor should not be able to switch root status")
	}
	if err := m.SwitchStatus(actor5Ctx, actor5.ID, "disable"); err != nil {
		t.Fatalf("restricted actor should switch its own status: %v", err)
	}
}

func TestAdminModelDeleteRejectsSubordinates(t *testing.T) {
	db := openMySQLAdminTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &AdminGroupAccess{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}, &AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)

	ctx := adminTestContext(t, true)
	m := NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

	root := createAdminForHierarchy(t, db, "root")
	child := createAdminForHierarchy(t, db, "child")
	h := newAdminHierarchy("ba_")
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := h.LinkNewNode(ctx.Request.Context(), tx, root.ID, nil); err != nil {
			return err
		}
		return h.LinkNewNode(ctx.Request.Context(), tx, child.ID, &root.ID)
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := m.Del(ctx, []int32{root.ID}); err == nil {
		t.Fatal("expected delete to be rejected because root has a subordinate")
	}

	// Verify closure rows are untouched.
	if got := countClosureRows(t, db); got != 3 {
		t.Fatalf("closure rows = %d, want 3", got)
	}
}

func TestAdminModelEditMoveFailureRollsBackPassword(t *testing.T) {
	db := openMySQLAdminTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&AdminClosure{}, &AdminGroupAccess{}, &Admin{})
	_ = db.Migrator().DropTable("ba_admin_hierarchy_lock")
	ensureLockTable(t, db)
	if err := db.AutoMigrate(&Admin{}, &AdminClosure{}, &AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)

	ctx := adminTestContext(t, true)
	m := NewAdminModel(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

	a := createAdminForHierarchy(t, db, "alice")
	h := newAdminHierarchy("ba_")
	if err := db.Transaction(func(tx *gorm.DB) error {
		return h.LinkNewNode(ctx.Request.Context(), tx, a.ID, nil)
	}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	oldHash := adminPasswordHash(t, db, a.ID)

	// Attempt to move under a non-existent parent while changing password.
	missing := int32(9999)
	updated := Admin{
		ID:       a.ID,
		Username: a.Username,
		Nickname: a.Nickname,
		Password: "new-password-123",
		Salt:     "new-salt-1234567",
		ParentID: &missing,
		Status:   "enable",
	}
	if err := m.Edit(ctx, updated, true, &missing, []string{"login_failure", "last_login_time", "last_login_ip"}, nil); err == nil {
		t.Fatal("expected edit to fail due to orphan parent")
	}

	newHash := adminPasswordHash(t, db, a.ID)
	if oldHash != newHash {
		t.Fatal("password was changed although the edit transaction rolled back")
	}
}

func adminPasswordHash(t *testing.T, db *gorm.DB, id int32) string {
	t.Helper()
	var hash string
	if err := db.Model(&Admin{}).Where("id = ?", id).Select("password").Scan(&hash).Error; err != nil {
		t.Fatalf("read password: %v", err)
	}
	return hash
}
