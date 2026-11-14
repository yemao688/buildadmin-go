package repository

import (
	"context"
	"testing"

	"buildadmin-go/internal/model"

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

func setupAdminDeleteChain(t *testing.T) (*gorm.DB, model.Admin, model.Admin, model.Admin) {
	t.Helper()
	db := openAdminDeleteTestDB(t)
	root := createAdminForHierarchy(t, db, "root")
	parent := createAdminForHierarchy(t, db, "parent")
	leaf := createAdminForHierarchy(t, db, "leaf")
	linkAdminForDelete(t, db, root.ID, nil)
	linkAdminForDelete(t, db, parent.ID, &root.ID)
	linkAdminForDelete(t, db, leaf.ID, &parent.ID)
	return db, root, parent, leaf
}

func countOwnerRows(t *testing.T, db *gorm.DB, table string, adminID int32) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where("admin_id = ?", adminID).Count(&count).Error)
	return count
}

func TestAdminHierarchyDeleteTargetsTxReturnsParentSnapshot(t *testing.T) {
	t.Parallel()
	db, root, parent, leaf := setupAdminDeleteChain(t)
	h := newAdminHierarchy("ba_")

	var targets []AdminDeleteTarget
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		targets, err = h.DeleteTargetsTx(context.Background(), tx, []int32{leaf.ID, root.ID})
		return err
	}))

	require.Len(t, targets, 2)
	require.Equal(t, AdminDeleteTarget{ID: root.ID}, targets[0], "targets are ordered by id")
	require.Equal(t, AdminDeleteTarget{ID: leaf.ID, ParentID: &parent.ID}, targets[1])
}

func TestAdminHierarchyDeleteTargetsTxRejectsMissingRows(t *testing.T) {
	t.Parallel()
	db, root, _, _ := setupAdminDeleteChain(t)
	h := newAdminHierarchy("ba_")

	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := h.DeleteTargetsTx(context.Background(), tx, []int32{root.ID, 999999})
		return err
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rows affected mismatch")
}

func TestAdminHierarchyCountSubordinatesTx(t *testing.T) {
	t.Parallel()
	db, root, parent, leaf := setupAdminDeleteChain(t)
	h := newAdminHierarchy("ba_")

	var counts map[int32]int64
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		counts = make(map[int32]int64, 3)
		for _, id := range []int32{root.ID, parent.ID, leaf.ID} {
			count, err := h.CountSubordinatesTx(context.Background(), tx, id)
			if err != nil {
				return err
			}
			counts[id] = count
		}
		return nil
	}))

	require.Equal(t, int64(2), counts[root.ID])
	require.Equal(t, int64(1), counts[parent.ID])
	require.Zero(t, counts[leaf.ID])
}

func TestAdminHierarchyAdminOwnsRowsTx(t *testing.T) {
	t.Parallel()
	db, _, _, leaf := setupAdminDeleteChain(t)
	createAdminOwnerTestTable(t, db, "ba_owner_a")
	require.NoError(t, db.Exec("INSERT INTO `ba_owner_a` (admin_id, update_time) VALUES (?,0)", leaf.ID).Error)

	// 未物化的登记表必须先跳过，已物化且有行的表才算拥有数据。
	h := &AdminHierarchy{prefix: "ba_", ownerTables: []string{"owner_missing", "owner_a"}}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		owns, err := h.AdminOwnsRowsTx(context.Background(), tx, leaf.ID)
		require.NoError(t, err)
		require.True(t, owns)

		owns, err = h.AdminOwnsRowsTx(context.Background(), tx, 999999)
		require.NoError(t, err)
		require.False(t, owns)
		return nil
	}))
}

func TestAdminHierarchyEnsureReassignmentTargetEnabledTx(t *testing.T) {
	t.Parallel()
	db, root, parent, _ := setupAdminDeleteChain(t)
	h := newAdminHierarchy("ba_")
	require.NoError(t, db.Model(&model.Admin{}).Where("id = ?", parent.ID).Update("status", "disable").Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return h.EnsureReassignmentTargetEnabledTx(context.Background(), tx, root.ID)
	}))

	err := db.Transaction(func(tx *gorm.DB) error {
		return h.EnsureReassignmentTargetEnabledTx(context.Background(), tx, parent.ID)
	})
	require.ErrorIs(t, err, ErrReassignmentTargetDisabled)

	err = db.Transaction(func(tx *gorm.DB) error {
		return h.EnsureReassignmentTargetEnabledTx(context.Background(), tx, 999999)
	})
	require.ErrorIs(t, err, ErrHierarchyNodeNotFound)
}

func TestAdminHierarchyTransferOwnerRowsTx(t *testing.T) {
	t.Parallel()
	db, _, parent, leaf := setupAdminDeleteChain(t)
	createAdminOwnerTestTable(t, db, "ba_owner_b")
	require.NoError(t, db.Exec("INSERT INTO `ba_owner_b` (admin_id, update_time) VALUES (?,0),(?,0)", leaf.ID, parent.ID).Error)

	h := newAdminHierarchy("ba_")
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		rows, err := h.TransferOwnerRowsTx(context.Background(), tx, "owner_b", leaf.ID, parent.ID)
		require.NoError(t, err)
		require.Equal(t, int64(1), rows)

		// 未物化的登记表没有行可转移，不能报错。
		rows, err = h.TransferOwnerRowsTx(context.Background(), tx, "owner_missing", leaf.ID, parent.ID)
		require.NoError(t, err)
		require.Zero(t, rows)
		return nil
	}))

	require.Equal(t, int64(2), countOwnerRows(t, db, "ba_owner_b", parent.ID))
	require.Zero(t, countOwnerRows(t, db, "ba_owner_b", leaf.ID))

	var updateTime int64
	require.NoError(t, db.Table("ba_owner_b").Select("MAX(update_time)").Where("admin_id = ?", parent.ID).Scan(&updateTime).Error)
	require.NotZero(t, updateTime, "transferred row must refresh update_time")
}

func TestAdminHierarchyDeleteAdminsTxGuardsRowsAffected(t *testing.T) {
	t.Parallel()
	db, _, _, leaf := setupAdminDeleteChain(t)
	h := newAdminHierarchy("ba_")

	err := db.Transaction(func(tx *gorm.DB) error {
		return h.DeleteAdminsTx(context.Background(), tx, []int32{leaf.ID, 999999})
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "rows affected mismatch")

	var adminCount int64
	require.NoError(t, db.Model(&model.Admin{}).Where("id = ?", leaf.ID).Count(&adminCount).Error)
	require.Equal(t, int64(1), adminCount, "mismatched delete must roll back")

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return h.DeleteAdminsTx(context.Background(), tx, []int32{leaf.ID})
	}))
	require.NoError(t, db.Model(&model.Admin{}).Where("id = ?", leaf.ID).Count(&adminCount).Error)
	require.Zero(t, adminCount)
}

func TestAdminHierarchyDeleteClosureTx(t *testing.T) {
	t.Parallel()
	db, root, _, leaf := setupAdminDeleteChain(t)
	h := newAdminHierarchy("ba_")

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return h.DeleteClosureTx(context.Background(), tx, []int32{leaf.ID})
	}))
	var count int64
	require.NoError(t, db.Model(&model.AdminClosure{}).Where("descendant_id = ?", leaf.ID).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Model(&model.AdminClosure{}).Where("descendant_id = ?", root.ID).Count(&count).Error)
	require.Equal(t, int64(1), count, "ancestor closure rows must survive")
}
