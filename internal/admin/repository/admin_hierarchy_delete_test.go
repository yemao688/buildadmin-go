package repository

import (
	"context"
	"testing"

	"buildadmin-go/internal/model"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openAdminDeleteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := openAdminHierarchyTestDB(t, "ba_")
	_ = db.Migrator().DropTable(&model.AdminClosure{}, &model.Admin{})
	require.NoError(t, db.AutoMigrate(&model.Admin{}, &model.AdminClosure{}))
	return db
}

func createAdminOwnerTestTable(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS `"+table+"`").Error)
	require.NoError(t, db.Exec("CREATE TABLE `"+table+"` (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL, update_time BIGINT NULL)").Error)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS `" + table + "`").Error
	})
}

func linkAdminForDelete(t *testing.T, db *gorm.DB, nodeID int32, parentID *int32) {
	t.Helper()
	h := newAdminHierarchy("ba_")
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return h.LinkNewNode(context.Background(), tx, nodeID, parentID)
	}))
}

func deleteAdminsForTest(t *testing.T, db *gorm.DB, h *AdminHierarchy, ids []int32, actorID int32) (*AdminDeleteResult, error) {
	t.Helper()
	actor := adminDelActorForRepository(t, actorID)
	var result *AdminDeleteResult
	err := db.Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = h.DeleteAdminsWithResult(context.Background(), tx, ids, actor, nil)
		return txErr
	})
	return result, err
}

func adminDelActorForRepository(t *testing.T, id int32) data_scope.Actor {
	t.Helper()
	actor, err := data_scope.NewUnrestrictedActor(id)
	require.NoError(t, err)
	return actor
}

func countOwnerRows(t *testing.T, db *gorm.DB, table string, adminID int32) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where("admin_id = ?", adminID).Count(&count).Error)
	return count
}

func TestAdminHierarchyDeleteTransfersOwnersToParent(t *testing.T) {
	db := openAdminDeleteTestDB(t)
	root := createAdminForHierarchy(t, db, "root")
	parent := createAdminForHierarchy(t, db, "parent")
	leaf := createAdminForHierarchy(t, db, "leaf")
	linkAdminForDelete(t, db, root.ID, nil)
	linkAdminForDelete(t, db, parent.ID, &root.ID)
	linkAdminForDelete(t, db, leaf.ID, &parent.ID)

	createAdminOwnerTestTable(t, db, "ba_owner_a")
	require.NoError(t, db.Exec("INSERT INTO `ba_owner_a` (admin_id, update_time) VALUES (?,0),(?,0)", leaf.ID, parent.ID).Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS `ba_admin_log`").Error)
	require.NoError(t, db.Exec("CREATE TABLE `ba_admin_log` (id INT AUTO_INCREMENT PRIMARY KEY, admin_id INT NOT NULL)").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS `ba_admin_log`").Error })
	require.NoError(t, db.Exec("INSERT INTO `ba_admin_log` (admin_id) VALUES (?)", leaf.ID).Error)

	h := &AdminHierarchy{prefix: "ba_", ownerTables: []string{"owner_a"}}
	result, err := deleteAdminsForTest(t, db, h, []int32{leaf.ID}, root.ID)
	require.NoError(t, err)

	require.Equal(t, []int32{leaf.ID}, result.Deleted)
	require.Len(t, result.Transfers, 1)
	require.Equal(t, AdminOwnerTransfer{
		AdminID:    leaf.ID,
		NewOwnerID: parent.ID,
		Table:      "owner_a",
		Rows:       1,
	}, result.Transfers[0])
	require.Equal(t, int64(2), countOwnerRows(t, db, "ba_owner_a", parent.ID))
	require.Zero(t, countOwnerRows(t, db, "ba_owner_a", leaf.ID))

	var updateTime int64
	require.NoError(t, db.Table("ba_owner_a").Select("MAX(update_time)").Where("admin_id = ?", parent.ID).Scan(&updateTime).Error)
	require.NotZero(t, updateTime, "transferred row must refresh update_time")

	require.Equal(t, int64(1), countOwnerRows(t, db, "ba_admin_log", leaf.ID), "unregistered audit table must not be transferred")
	var adminCount int64
	require.NoError(t, db.Model(&model.Admin{}).Where("id = ?", leaf.ID).Count(&adminCount).Error)
	require.Zero(t, adminCount)
	var closureCount int64
	require.NoError(t, db.Model(&model.AdminClosure{}).Where("descendant_id = ?", leaf.ID).Count(&closureCount).Error)
	require.Zero(t, closureCount)
}

func TestAdminHierarchyDeleteTransfersEachLeafToItsOwnParent(t *testing.T) {
	db := openAdminDeleteTestDB(t)
	root := createAdminForHierarchy(t, db, "root")
	parentA := createAdminForHierarchy(t, db, "parent-a")
	parentB := createAdminForHierarchy(t, db, "parent-b")
	leafA := createAdminForHierarchy(t, db, "leaf-a")
	leafB := createAdminForHierarchy(t, db, "leaf-b")
	linkAdminForDelete(t, db, root.ID, nil)
	linkAdminForDelete(t, db, parentA.ID, &root.ID)
	linkAdminForDelete(t, db, parentB.ID, &root.ID)
	linkAdminForDelete(t, db, leafA.ID, &parentA.ID)
	linkAdminForDelete(t, db, leafB.ID, &parentB.ID)

	createAdminOwnerTestTable(t, db, "ba_owner_b")
	require.NoError(t, db.Exec("INSERT INTO `ba_owner_b` (admin_id) VALUES (?),(?)", leafA.ID, leafB.ID).Error)

	h := &AdminHierarchy{prefix: "ba_", ownerTables: []string{"owner_b"}}
	result, err := deleteAdminsForTest(t, db, h, []int32{leafB.ID, leafA.ID}, root.ID)
	require.NoError(t, err)

	require.Equal(t, []int32{leafA.ID, leafB.ID}, result.Deleted)
	require.Len(t, result.Transfers, 2)
	require.Equal(t, int64(1), countOwnerRows(t, db, "ba_owner_b", parentA.ID))
	require.Equal(t, int64(1), countOwnerRows(t, db, "ba_owner_b", parentB.ID))
	require.Zero(t, countOwnerRows(t, db, "ba_owner_b", leafA.ID))
	require.Zero(t, countOwnerRows(t, db, "ba_owner_b", leafB.ID))
}

func TestAdminHierarchyDeleteRejectsTopLevelAdmin(t *testing.T) {
	db := openAdminDeleteTestDB(t)
	root := createAdminForHierarchy(t, db, "root")
	linkAdminForDelete(t, db, root.ID, nil)

	createAdminOwnerTestTable(t, db, "ba_owner_c")
	require.NoError(t, db.Exec("INSERT INTO `ba_owner_c` (admin_id) VALUES (?)", root.ID).Error)

	h := &AdminHierarchy{prefix: "ba_", ownerTables: []string{"owner_c"}}
	result, err := deleteAdminsForTest(t, db, h, []int32{root.ID}, root.ID)
	require.Nil(t, result)
	var businessErr *cErr.Error
	require.ErrorAs(t, err, &businessErr)

	var adminCount int64
	require.NoError(t, db.Model(&model.Admin{}).Where("id = ?", root.ID).Count(&adminCount).Error)
	require.Equal(t, int64(1), adminCount)
	require.Equal(t, int64(1), countOwnerRows(t, db, "ba_owner_c", root.ID))
}

func TestAdminHierarchyDeleteTopLevelWithoutOwnedRows(t *testing.T) {
	db := openAdminDeleteTestDB(t)
	root := createAdminForHierarchy(t, db, "root")
	linkAdminForDelete(t, db, root.ID, nil)

	// 升级库的 admin.parent_id 是 NULL，登记表存在但没有任何该管理员的数据。
	createAdminOwnerTestTable(t, db, "ba_owner_e")

	h := &AdminHierarchy{prefix: "ba_", ownerTables: []string{"owner_e"}}
	result, err := deleteAdminsForTest(t, db, h, []int32{root.ID}, root.ID)
	require.NoError(t, err)
	require.Equal(t, []int32{root.ID}, result.Deleted)
	require.Empty(t, result.Transfers)

	var adminCount int64
	require.NoError(t, db.Model(&model.Admin{}).Where("id = ?", root.ID).Count(&adminCount).Error)
	require.Zero(t, adminCount)
	var closureCount int64
	require.NoError(t, db.Model(&model.AdminClosure{}).Where("descendant_id = ?", root.ID).Count(&closureCount).Error)
	require.Zero(t, closureCount)
}

func TestAdminHierarchyDeleteRejectsDisabledReassignmentTarget(t *testing.T) {
	db := openAdminDeleteTestDB(t)
	root := createAdminForHierarchy(t, db, "root")
	parent := createAdminForHierarchy(t, db, "parent")
	leaf := createAdminForHierarchy(t, db, "leaf")
	linkAdminForDelete(t, db, root.ID, nil)
	linkAdminForDelete(t, db, parent.ID, &root.ID)
	linkAdminForDelete(t, db, leaf.ID, &parent.ID)
	require.NoError(t, db.Model(&model.Admin{}).Where("id = ?", parent.ID).Update("status", "disable").Error)

	createAdminOwnerTestTable(t, db, "ba_owner_f")
	require.NoError(t, db.Exec("INSERT INTO `ba_owner_f` (admin_id) VALUES (?)", leaf.ID).Error)

	h := &AdminHierarchy{prefix: "ba_", ownerTables: []string{"owner_f"}}
	result, err := deleteAdminsForTest(t, db, h, []int32{leaf.ID}, root.ID)
	require.Nil(t, result)
	var businessErr *cErr.Error
	require.ErrorAs(t, err, &businessErr)
	require.Contains(t, err.Error(), "is not enabled")

	require.Equal(t, int64(1), countOwnerRows(t, db, "ba_owner_f", leaf.ID))
	var adminCount int64
	require.NoError(t, db.Model(&model.Admin{}).Where("id = ?", leaf.ID).Count(&adminCount).Error)
	require.Equal(t, int64(1), adminCount)
}

func TestAdminHierarchyDeleteRollsBackOwnerTransfer(t *testing.T) {
	db := openAdminDeleteTestDB(t)
	root := createAdminForHierarchy(t, db, "root")
	parent := createAdminForHierarchy(t, db, "parent")
	leaf := createAdminForHierarchy(t, db, "leaf")
	linkAdminForDelete(t, db, root.ID, nil)
	linkAdminForDelete(t, db, parent.ID, &root.ID)
	linkAdminForDelete(t, db, leaf.ID, &parent.ID)

	createAdminOwnerTestTable(t, db, "ba_owner_d")
	require.NoError(t, db.Exec("INSERT INTO `ba_owner_d` (admin_id) VALUES (?)", leaf.ID).Error)
	trigger := "`ba_admin_closure_delete_block`"
	require.NoError(t, db.Exec("DROP TRIGGER IF EXISTS "+trigger).Error)
	require.NoError(t, db.Exec("CREATE TRIGGER "+trigger+" BEFORE DELETE ON `ba_admin_closure` FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'closure delete blocked'").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TRIGGER IF EXISTS " + trigger).Error })

	h := &AdminHierarchy{prefix: "ba_", ownerTables: []string{"owner_d"}}
	_, err := deleteAdminsForTest(t, db, h, []int32{leaf.ID}, root.ID)
	require.Error(t, err)
	require.Equal(t, int64(1), countOwnerRows(t, db, "ba_owner_d", leaf.ID), "owner transfer must roll back with the failed delete")
	require.Zero(t, countOwnerRows(t, db, "ba_owner_d", parent.ID))

	var adminCount int64
	require.NoError(t, db.Model(&model.Admin{}).Where("id = ?", leaf.ID).Count(&adminCount).Error)
	require.Equal(t, int64(1), adminCount)
}
