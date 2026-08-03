package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	adminmodel "buildadmin-go/internal/admin/repository"
	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	"buildadmin-go/internal/pkg/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// adminDelFixture wires the AdminService delete flow against the shared
// mysql_test database with an isolated per-test table prefix (concurrent
// packages must never share ba_ tables).
type adminDelFixture struct {
	db     *gorm.DB
	config *conf.Configuration
	svc    *AdminService
	prefix string
}

func newAdminDelFixture(t *testing.T) *adminDelFixture {
	t.Helper()
	prefix := fmt.Sprintf("it_%d_", time.Now().UnixNano())
	config := &conf.Configuration{Database: conf.Database{Prefix: prefix}}
	db, _ := testutil.OpenMySQL(t)
	db.Config.NamingStrategy = schema.NamingStrategy{SingularTable: true, TablePrefix: prefix}
	_ = db.Exec("DROP TRIGGER IF EXISTS `" + prefix + "admin_closure_delete_block`")
	_ = db.Migrator().DropTable(&model.AdminClosure{}, &model.AdminGroupAccess{}, &model.Admin{})
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.AdminClosure{}, &model.AdminGroupAccess{}))
	require.NoError(t, db.Exec("ALTER TABLE `"+prefix+"admin` ALTER COLUMN `login_failure` SET DEFAULT 0").Error)
	require.NoError(t, db.Exec("ALTER TABLE `"+prefix+"admin` MODIFY COLUMN `last_login_ip` VARCHAR(50) NOT NULL DEFAULT ''").Error)
	t.Cleanup(func() {
		_ = db.Exec("DROP TRIGGER IF EXISTS `" + prefix + "admin_closure_delete_block`").Error
		_ = db.Migrator().DropTable(&model.Admin{}, &model.AdminClosure{}, &model.AdminGroupAccess{})
	})
	adminM := adminmodel.NewAdminRepository(db, config)
	authM := adminmodel.NewAuthRepository(db, nil, config)
	return &adminDelFixture{db: db, config: config, svc: NewAdminService(adminM, authM, config), prefix: prefix}
}

func (f *adminDelFixture) createAdmin(t *testing.T, username string) model.Admin {
	t.Helper()
	a := model.Admin{Username: username, Status: "enable"}
	require.NoError(t, f.db.Create(&a).Error)
	return a
}

func (f *adminDelFixture) link(t *testing.T, nodeID int32, parentID *int32) {
	t.Helper()
	h := adminmodel.NewAdminHierarchy(f.config)
	require.NoError(t, f.db.Transaction(func(tx *gorm.DB) error {
		return h.LinkNewNode(context.Background(), tx, nodeID, parentID)
	}))
}

func adminDelActor(t *testing.T, id int32, unrestricted bool) data_scope.Actor {
	t.Helper()
	if unrestricted {
		a, err := data_scope.NewUnrestrictedActor(id)
		require.NoError(t, err)
		return a
	}
	a, err := data_scope.NewActor(id)
	require.NoError(t, err)
	return a
}

func TestRestrictedDeleteVisibleLeafAndRejectInvisibleSibling(t *testing.T) {
	f := newAdminDelFixture(t)
	root := f.createAdmin(t, "root")
	actor := f.createAdmin(t, "actor")
	leaf := f.createAdmin(t, "leaf")
	sibling := f.createAdmin(t, "sibling")
	f.link(t, root.ID, nil)
	f.link(t, actor.ID, &root.ID)
	f.link(t, leaf.ID, &actor.ID)
	f.link(t, sibling.ID, &root.ID)

	actorCtx := adminDelActor(t, actor.ID, false)
	require.NoError(t, f.svc.Del(context.Background(), []int32{leaf.ID, leaf.ID}, actorCtx))
	var count int64
	f.db.Model(&model.Admin{}).Where("id = ?", leaf.ID).Count(&count)
	require.Zero(t, count, "leaf admin row was not deleted")
	f.db.Model(&model.AdminClosure{}).Where("descendant_id = ?", leaf.ID).Count(&count)
	require.Zero(t, count, "leaf closure rows were not deleted")
	require.Error(t, f.svc.Del(context.Background(), []int32{sibling.ID}, actorCtx), "restricted actor deleted an invisible sibling")
	f.db.Model(&model.Admin{}).Where("id = ?", sibling.ID).Count(&count)
	require.Equal(t, int64(1), count, "invisible sibling was deleted")
}

func TestDeleteClosureFailureRollsBackAdminAndClosure(t *testing.T) {
	f := newAdminDelFixture(t)
	actor := f.createAdmin(t, "actor")
	leaf := f.createAdmin(t, "leaf")
	f.link(t, actor.ID, nil)
	f.link(t, leaf.ID, &actor.ID)
	trigger := "`" + f.prefix + "admin_closure_delete_block`"
	require.NoError(t, f.db.Exec("CREATE TRIGGER "+trigger+" BEFORE DELETE ON `"+f.prefix+"admin_closure` FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'closure delete blocked'").Error)

	require.Error(t, f.svc.Del(context.Background(), []int32{leaf.ID}, adminDelActor(t, actor.ID, false)))
	var count int64
	f.db.Model(&model.Admin{}).Where("id = ?", leaf.ID).Count(&count)
	require.Equal(t, int64(1), count, "admin delete was not rolled back")
	f.db.Model(&model.AdminClosure{}).Where("descendant_id = ?", leaf.ID).Count(&count)
	require.NotZero(t, count, "closure delete was not rolled back")
}

func TestAdminModelDeleteRejectsSubordinates(t *testing.T) {
	f := newAdminDelFixture(t)
	root := f.createAdmin(t, "root")
	child := f.createAdmin(t, "child")
	f.link(t, root.ID, nil)
	f.link(t, child.ID, &root.ID)

	require.Error(t, f.svc.Del(context.Background(), []int32{root.ID}, adminDelActor(t, root.ID, true)), "delete must be rejected because root has a subordinate")
	var count int64
	f.db.Model(&model.AdminClosure{}).Count(&count)
	require.Equal(t, int64(3), count, "closure rows = %d, want 3", count)
}

func TestNormalizeAdminIDs(t *testing.T) {
	got, err := normalizeAdminIDs([]int32{4, 4, 2, 4})
	require.NoError(t, err)
	require.Equal(t, []int32{4, 2}, got)
	for _, ids := range [][]int32{{0}, {-1}, {2, 0, 3}} {
		_, err := normalizeAdminIDs(ids)
		require.Error(t, err, "normalizeAdminIDs(%v) should reject non-positive IDs", ids)
	}
}
