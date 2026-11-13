package repository

import (
	"errors"
	"testing"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/requesttx"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func openMySQLAdminTestDB(t *testing.T, prefix string) *gorm.DB {
	t.Helper()
	db, _ := testutil.OpenMySQL(t)
	db.Config.NamingStrategy = schema.NamingStrategy{
		SingularTable: true,
		TablePrefix:   prefix,
	}
	return db
}

func prepareAdminModelMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	db := openMySQLAdminTestDB(t, "ba_")
	_ = db.Exec("DROP TRIGGER IF EXISTS ba_admin_closure_delete_block")
	_ = db.Migrator().DropTable(&model.AdminClosure{}, &model.AdminGroupAccess{}, &model.Admin{})
	if err := db.AutoMigrate(&model.Admin{}, &model.AdminClosure{}, &model.AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)
	return db
}

func linkAdmins(t *testing.T, db *gorm.DB, links ...struct {
	node   model.Admin
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

func TestAdminAddActiveRequestTransactionRollsBackWithOuterFailure(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	m := NewAdminRepository(db, hierarchyConfig("ba_"))
	ctx := adminTestContext(t, true)
	missingParent := int32(999999)
	admin := model.Admin{ParentID: &missingParent, Username: "requesttx-failure", Status: "enable"}

	err := db.Transaction(func(tx *gorm.DB) error {
		ctx.Request = ctx.Request.WithContext(requesttx.Bind(ctx.Request.Context(), tx))
		if err := m.AddWithActor(ctx.Request.Context(), admin, nil, actorForContext(t, ctx)); err == nil {
			t.Fatal("expected hierarchy validation failure")
		}
		return errors.New("outer business failure")
	})
	if err == nil {
		t.Fatal("expected outer transaction failure")
	}

	var count int64
	if err := db.Model(&model.Admin{}).Where("username = ?", admin.Username).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed request transaction leaked %d admin rows", count)
	}
}

func TestAdminAddWithoutRequestTransactionCommitsOwnTransaction(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	m := NewAdminRepository(db, hierarchyConfig("ba_"))
	ctx := adminTestContext(t, true)
	admin := model.Admin{Username: "requesttx-success", Status: "enable"}

	if err := m.AddWithActor(ctx.Request.Context(), admin, nil, actorForContext(t, ctx)); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.Admin{}).Where("username = ?", admin.Username).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected committed admin row, got %d", count)
	}
}

func TestAdminAddDuplicateUsernameReturnsFriendlyError(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	m := NewAdminRepository(db, hierarchyConfig("ba_"))
	ctx := adminTestContext(t, true)
	admin := model.Admin{Username: "duplicate-username", Nickname: "first", Status: "enable"}
	require.NoError(t, m.AddWithActor(ctx.Request.Context(), admin, nil, actorForContext(t, ctx)))
	require.ErrorContains(t, m.AddWithActor(ctx.Request.Context(), model.Admin{Username: admin.Username, Nickname: "second", Status: "enable"}, nil, actorForContext(t, ctx)), "username already exists")
}

func TestRestrictedHierarchyWritersRejectRoot(t *testing.T) {
	db := prepareAdminModelMySQL(t)
	actorAdmin := createAdminForHierarchy(t, db, "actor")
	candidate := createAdminForHierarchy(t, db, "candidate")
	linkAdmins(t, db, struct {
		node   model.Admin
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
		node   model.Admin
		parent *int32
	}{invisibleParent, nil}, struct {
		node   model.Admin
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
	if err := db.Model(&model.Admin{}).Where("id = ?", actorAdmin.ID).Update("parent_id", nil).Error; err != nil {
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
	_ = db.Migrator().DropTable(&model.AdminClosure{}, &model.AdminGroupAccess{}, &model.Admin{})
	if err := db.AutoMigrate(&model.Admin{}, &model.AdminClosure{}, &model.AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)

	ctx := adminTestContext(t, true)
	m := NewAdminRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
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
	_ = db.Migrator().DropTable(&model.AdminClosure{}, &model.AdminGroupAccess{}, &model.Admin{})
	if err := db.AutoMigrate(&model.Admin{}, &model.AdminClosure{}, &model.AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)

	m := NewAdminRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})
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
	if err := m.SwitchStatusWithActor(actor5Ctx.Request.Context(), root.ID, "disable", actorForContext(t, actor5Ctx)); err == nil {
		t.Fatal("restricted actor should not be able to switch root status")
	}
	if err := m.SwitchStatusWithActor(actor5Ctx.Request.Context(), actor5.ID, "disable", actorForContext(t, actor5Ctx)); err != nil {
		t.Fatalf("restricted actor should switch its own status: %v", err)
	}
}

func TestAdminModelEditMoveFailureRollsBackPassword(t *testing.T) {
	db := openMySQLAdminTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&model.AdminClosure{}, &model.AdminGroupAccess{}, &model.Admin{})
	if err := db.AutoMigrate(&model.Admin{}, &model.AdminClosure{}, &model.AdminGroupAccess{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	ensureAdminDefaults(t, db)

	ctx := adminTestContext(t, true)
	m := NewAdminRepository(db, &conf.Configuration{Database: conf.Database{Prefix: "ba_"}})

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
	updated := model.Admin{
		ID:       a.ID,
		Username: a.Username,
		Nickname: a.Nickname,
		Password: "new-password-123",
		ParentID: &missing,
		Status:   "enable",
	}
	if err := m.EditWithActor(ctx.Request.Context(), updated, true, &missing, []string{"login_failure", "last_login_time", "last_login_ip"}, nil, actorForContext(t, ctx)); err == nil {
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
	if err := db.Model(&model.Admin{}).Where("id = ?", id).Select("password").Scan(&hash).Error; err != nil {
		t.Fatalf("read password: %v", err)
	}
	return hash
}
