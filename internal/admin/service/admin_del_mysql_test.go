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
	cErr "buildadmin-go/internal/pkg/error"
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

// createFrameworkOwnerTables 建立框架默认登记的两张归属表，验证删除叶子
// 管理员时 user / user_money_log 在同一事务内被转挂到上级。
func (f *adminDelFixture) createFrameworkOwnerTables(t *testing.T) {
	t.Helper()
	require.NoError(t, f.db.Exec("DROP TABLE IF EXISTS `"+f.prefix+"user`").Error)
	require.NoError(t, f.db.Exec("DROP TABLE IF EXISTS `"+f.prefix+"user_money_log`").Error)
	require.NoError(t, f.db.Exec("CREATE TABLE `"+f.prefix+"user` (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, update_time BIGINT NULL)").Error)
	require.NoError(t, f.db.Exec("CREATE TABLE `"+f.prefix+"user_money_log` (id INT AUTO_INCREMENT PRIMARY KEY, user_id INT NOT NULL, admin_id INT NOT NULL, update_time BIGINT NULL)").Error)
	t.Cleanup(func() {
		_ = f.db.Exec("DROP TABLE IF EXISTS `" + f.prefix + "user`").Error
		_ = f.db.Exec("DROP TABLE IF EXISTS `" + f.prefix + "user_money_log`").Error
	})
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

func TestAdminDeleteRestrictedVisibleLeafAndRejectInvisibleSibling(t *testing.T) {
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

func TestAdminDeleteClosureFailureRollsBackAdminAndClosure(t *testing.T) {
	f := newAdminDelFixture(t)
	f.createFrameworkOwnerTables(t)
	actor := f.createAdmin(t, "actor")
	leaf := f.createAdmin(t, "leaf")
	f.link(t, actor.ID, nil)
	f.link(t, leaf.ID, &actor.ID)
	require.NoError(t, f.db.Exec("INSERT INTO `"+f.prefix+"user` (admin_id, update_time) VALUES (?,0)", leaf.ID).Error)
	trigger := "`" + f.prefix + "admin_closure_delete_block`"
	require.NoError(t, f.db.Exec("CREATE TRIGGER "+trigger+" BEFORE DELETE ON `"+f.prefix+"admin_closure` FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'closure delete blocked'").Error)

	require.Error(t, f.svc.Del(context.Background(), []int32{leaf.ID}, adminDelActor(t, actor.ID, false)))
	var count int64
	f.db.Model(&model.Admin{}).Where("id = ?", leaf.ID).Count(&count)
	require.Equal(t, int64(1), count, "admin delete was not rolled back")
	f.db.Model(&model.AdminClosure{}).Where("descendant_id = ?", leaf.ID).Count(&count)
	require.NotZero(t, count, "closure delete was not rolled back")
	f.db.Table(f.prefix+"user").Where("admin_id = ?", leaf.ID).Count(&count)
	require.Equal(t, int64(1), count, "owner transfer was not rolled back")
}

func TestAdminDeleteLeafTransfersFrameworkOwnerRows(t *testing.T) {
	f := newAdminDelFixture(t)
	f.createFrameworkOwnerTables(t)
	root := f.createAdmin(t, "root")
	actor := f.createAdmin(t, "actor")
	leaf := f.createAdmin(t, "leaf")
	f.link(t, root.ID, nil)
	f.link(t, actor.ID, &root.ID)
	f.link(t, leaf.ID, &actor.ID)
	require.NoError(t, f.db.Exec("INSERT INTO `"+f.prefix+"user` (id, admin_id, update_time) VALUES (100, ?, 0)", leaf.ID).Error)
	require.NoError(t, f.db.Exec("INSERT INTO `"+f.prefix+"user_money_log` (user_id, admin_id, update_time) VALUES (100, ?, 0)", leaf.ID).Error)

	require.NoError(t, f.svc.Del(context.Background(), []int32{leaf.ID}, adminDelActor(t, actor.ID, false)))

	var userCount int64
	f.db.Table(f.prefix+"user").Where("admin_id = ?", actor.ID).Count(&userCount)
	require.Equal(t, int64(1), userCount, "user owner was not transferred to the parent")
	f.db.Table(f.prefix+"user_money_log").Where("admin_id = ?", actor.ID).Count(&userCount)
	require.Equal(t, int64(1), userCount, "user_money_log owner was not transferred with user")
	f.db.Table(f.prefix+"user_money_log").Where("admin_id = ?", leaf.ID).Count(&userCount)
	require.Zero(t, userCount)
}

func TestAdminDeleteRejectsSubordinates(t *testing.T) {
	f := newAdminDelFixture(t)
	root := f.createAdmin(t, "root")
	child := f.createAdmin(t, "child")
	f.link(t, root.ID, nil)
	f.link(t, child.ID, &root.ID)

	err := f.svc.Del(context.Background(), []int32{root.ID}, adminDelActor(t, root.ID, true))
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot delete administrator with subordinates")
	var count int64
	f.db.Model(&model.AdminClosure{}).Count(&count)
	require.Equal(t, int64(3), count, "closure rows = %d, want 3", count)
}

func TestAdminDeleteTopLevelWithoutOwnedRows(t *testing.T) {
	f := newAdminDelFixture(t)
	f.createFrameworkOwnerTables(t)
	root := f.createAdmin(t, "root")
	f.link(t, root.ID, nil)

	require.NoError(t, f.svc.Del(context.Background(), []int32{root.ID}, adminDelActor(t, root.ID, true)))

	var count int64
	f.db.Model(&model.Admin{}).Where("id = ?", root.ID).Count(&count)
	require.Zero(t, count, "top-level admin without owned rows must be deletable")
	f.db.Model(&model.AdminClosure{}).Where("descendant_id = ?", root.ID).Count(&count)
	require.Zero(t, count)
}

func TestAdminDeleteTopLevelWithOwnedRowsRejected(t *testing.T) {
	f := newAdminDelFixture(t)
	f.createFrameworkOwnerTables(t)
	root := f.createAdmin(t, "root")
	f.link(t, root.ID, nil)
	require.NoError(t, f.db.Exec("INSERT INTO `"+f.prefix+"user` (admin_id, update_time) VALUES (?,0)", root.ID).Error)

	err := f.svc.Del(context.Background(), []int32{root.ID}, adminDelActor(t, root.ID, true))
	var businessErr *cErr.Error
	require.ErrorAs(t, err, &businessErr)
	require.Contains(t, err.Error(), "cannot delete top-level administrator")
	require.Contains(t, err.Error(), "without a reassignment target")

	var count int64
	f.db.Model(&model.Admin{}).Where("id = ?", root.ID).Count(&count)
	require.Equal(t, int64(1), count)
	f.db.Table(f.prefix+"user").Where("admin_id = ?", root.ID).Count(&count)
	require.Equal(t, int64(1), count, "owner row must not move without a target")
}

func TestAdminDeleteRejectsDisabledReassignmentTarget(t *testing.T) {
	f := newAdminDelFixture(t)
	f.createFrameworkOwnerTables(t)
	root := f.createAdmin(t, "root")
	parent := f.createAdmin(t, "parent")
	leaf := f.createAdmin(t, "leaf")
	f.link(t, root.ID, nil)
	f.link(t, parent.ID, &root.ID)
	f.link(t, leaf.ID, &parent.ID)
	require.NoError(t, f.db.Model(&model.Admin{}).Where("id = ?", parent.ID).Update("status", "disable").Error)
	require.NoError(t, f.db.Exec("INSERT INTO `"+f.prefix+"user` (admin_id, update_time) VALUES (?,0)", leaf.ID).Error)

	err := f.svc.Del(context.Background(), []int32{leaf.ID}, adminDelActor(t, root.ID, true))
	var businessErr *cErr.Error
	require.ErrorAs(t, err, &businessErr)
	require.Contains(t, err.Error(), "reassignment target administrator")

	var count int64
	f.db.Table(f.prefix+"user").Where("admin_id = ?", leaf.ID).Count(&count)
	require.Equal(t, int64(1), count, "owner rows must stay when the target is disabled")
	f.db.Model(&model.Admin{}).Where("id = ?", leaf.ID).Count(&count)
	require.Equal(t, int64(1), count)
}

// 正常闭包下父级有下级会被子级检查先拒绝；直接改写 admin.parent_id 模拟
// 闭包与列不一致的异常库，覆盖"父级也在删除集合"的防御分支。
func TestAdminDeleteRejectsParentInSameBatch(t *testing.T) {
	f := newAdminDelFixture(t)
	f.createFrameworkOwnerTables(t)
	root := f.createAdmin(t, "root")
	first := f.createAdmin(t, "first")
	second := f.createAdmin(t, "second")
	f.link(t, root.ID, nil)
	f.link(t, first.ID, &root.ID)
	f.link(t, second.ID, &root.ID)
	require.NoError(t, f.db.Model(&model.Admin{}).Where("id = ?", first.ID).Update("parent_id", second.ID).Error)
	require.NoError(t, f.db.Exec("INSERT INTO `"+f.prefix+"user` (admin_id, update_time) VALUES (?,0)", first.ID).Error)

	err := f.svc.Del(context.Background(), []int32{first.ID, second.ID}, adminDelActor(t, root.ID, true))
	var businessErr *cErr.Error
	require.ErrorAs(t, err, &businessErr)
	require.Contains(t, err.Error(), "its parent")

	var count int64
	f.db.Table(f.prefix+"user").Where("admin_id = ?", first.ID).Count(&count)
	require.Equal(t, int64(1), count)
	f.db.Model(&model.Admin{}).Where("id IN ?", []int32{first.ID, second.ID}).Count(&count)
	require.Equal(t, int64(2), count)
}

func TestAdminDeleteAggregatesTransfersPerTargetAndTable(t *testing.T) {
	f := newAdminDelFixture(t)
	f.createFrameworkOwnerTables(t)
	root := f.createAdmin(t, "root")
	parentA := f.createAdmin(t, "parent-a")
	parentB := f.createAdmin(t, "parent-b")
	leafA := f.createAdmin(t, "leaf-a")
	leafB := f.createAdmin(t, "leaf-b")
	f.link(t, root.ID, nil)
	f.link(t, parentA.ID, &root.ID)
	f.link(t, parentB.ID, &root.ID)
	f.link(t, leafA.ID, &parentA.ID)
	f.link(t, leafB.ID, &parentB.ID)

	require.NoError(t, f.db.Exec("INSERT INTO `"+f.prefix+"user` (admin_id, update_time) VALUES (?,0),(?,0)", leafA.ID, leafB.ID).Error)
	require.NoError(t, f.db.Exec("INSERT INTO `"+f.prefix+"user_money_log` (user_id, admin_id, update_time) VALUES (1,?,0),(2,?,0)", leafA.ID, leafB.ID).Error)
	auditTable := f.prefix + "admin_log"
	require.NoError(t, f.db.Exec("DROP TABLE IF EXISTS `"+auditTable+"`").Error)
	require.NoError(t, f.db.Exec("CREATE TABLE `"+auditTable+"` (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL)").Error)
	t.Cleanup(func() { _ = f.db.Exec("DROP TABLE IF EXISTS `" + auditTable + "`").Error })
	require.NoError(t, f.db.Exec("INSERT INTO `"+auditTable+"` (admin_id) VALUES (?)", leafA.ID).Error)

	hierarchy := adminmodel.NewAdminHierarchy(f.config)
	enforcer := data_scope.NewClosureEnforcer(f.config)
	var result *adminmodel.AdminDeleteResult
	require.NoError(t, f.db.Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = f.svc.deleteAdminsTx(context.Background(), tx, hierarchy, []int32{leafB.ID, leafA.ID}, adminDelActor(t, root.ID, true), enforcer)
		return txErr
	}))

	require.Equal(t, []int32{leafA.ID, leafB.ID}, result.Deleted, "deleted ids are sorted")
	require.Equal(t, []adminmodel.AdminOwnerTransfer{
		{AdminID: leafA.ID, NewOwnerID: parentA.ID, Table: "user", Rows: 1},
		{AdminID: leafA.ID, NewOwnerID: parentA.ID, Table: "user_money_log", Rows: 1},
		{AdminID: leafB.ID, NewOwnerID: parentB.ID, Table: "user", Rows: 1},
		{AdminID: leafB.ID, NewOwnerID: parentB.ID, Table: "user_money_log", Rows: 1},
	}, result.Transfers)

	var count int64
	f.db.Table(f.prefix+"user").Where("admin_id = ?", parentA.ID).Count(&count)
	require.Equal(t, int64(1), count)
	f.db.Table(f.prefix+"user").Where("admin_id = ?", parentB.ID).Count(&count)
	require.Equal(t, int64(1), count)
	f.db.Table(auditTable).Where("admin_id = ?", leafA.ID).Count(&count)
	require.Equal(t, int64(1), count, "unregistered audit table must not be transferred")
}
