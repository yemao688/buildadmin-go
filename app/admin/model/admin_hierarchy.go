package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"go-build-admin/app/pkg/data_scope"
	"go-build-admin/conf"

	"gorm.io/gorm"
)

var _ data_scope.HierarchyWriter = (*AdminHierarchy)(nil)

var (
	ErrHierarchyNodeNotFound   = errors.New("admin hierarchy: node not found")
	ErrHierarchySelfMove       = errors.New("admin hierarchy: cannot move a node under itself")
	ErrHierarchyDescendantMove = errors.New("admin hierarchy: cannot move a node under its descendant")
	ErrHierarchyOrphanParent   = errors.New("admin hierarchy: parent does not exist")
	ErrHierarchyAlreadyLinked  = errors.New("admin hierarchy: node already linked")
	ErrHierarchyIntegrity      = errors.New("admin hierarchy: admin.parent_id and closure state inconsistent")
)

// AdminHierarchy maintains the admin_closure read model for the data-scope
// contract. Every mutating method receives a caller-owned transaction and never
// commits or rolls back.
type AdminHierarchy struct {
	prefix string
}

func NewAdminHierarchy(config *conf.Configuration) *AdminHierarchy {
	return &AdminHierarchy{prefix: config.Database.Prefix}
}

func (h *AdminHierarchy) adminTable() string   { return h.prefix + "admin" }
func (h *AdminHierarchy) closureTable() string { return h.prefix + "admin_closure" }
func (h *AdminHierarchy) lockTable() string    { return h.prefix + "admin_hierarchy_lock" }

func quoteIdentifier(value string) string {
	if value == "" {
		return "``"
	}
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func int32PtrEqual(a, b *int32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// lockHierarchy acquires a real SELECT ... FOR UPDATE lock on the single row
// of the independent admin_hierarchy_lock table. LinkNewNode and MoveSubtree
// share this lock so that all hierarchy mutations are serialized before any
// reads. The lock is released when the caller commits or rolls back the
// supplied transaction. If the lock row is missing the writer fails closed.
func (h *AdminHierarchy) lockHierarchy(ctx context.Context, tx *gorm.DB) error {
	var id uint8
	result := tx.WithContext(ctx).Raw(
		"SELECT id FROM " + quoteIdentifier(h.lockTable()) + " WHERE id = 1 FOR UPDATE",
	).Scan(&id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: hierarchy lock row missing", ErrHierarchyIntegrity)
	}
	return nil
}

func (h *AdminHierarchy) adminRowExists(ctx context.Context, tx *gorm.DB, id int32) error {
	var count int64
	if err := tx.WithContext(ctx).Table(h.adminTable()).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%w: admin %d", ErrHierarchyNodeNotFound, id)
	}
	return nil
}

func (h *AdminHierarchy) ensureNodeExists(ctx context.Context, tx *gorm.DB, id int32) error {
	var adminCount int64
	if err := tx.WithContext(ctx).Table(h.adminTable()).Where("id = ?", id).Count(&adminCount).Error; err != nil {
		return err
	}
	if adminCount == 0 {
		return fmt.Errorf("%w: admin %d", ErrHierarchyOrphanParent, id)
	}
	var closureCount int64
	if err := tx.WithContext(ctx).Table(h.closureTable()).
		Where("descendant_id = ? AND ancestor_id = ?", id, id).
		Count(&closureCount).Error; err != nil {
		return err
	}
	if closureCount == 0 {
		return fmt.Errorf("%w: closure self row %d", ErrHierarchyOrphanParent, id)
	}
	return nil
}

// LinkNewNode inserts the mandatory self-row and, when parentID is non-nil,
// copies all ancestor paths of the parent so the new node becomes a leaf under
// that parent. The writer validates node/parent existence, updates
// admin.parent_id, and writes the closure rows in the same caller-owned
// transaction.
func (h *AdminHierarchy) LinkNewNode(ctx context.Context, tx *gorm.DB, nodeID int32, parentID *int32) error {
	if nodeID <= 0 {
		return fmt.Errorf("%w: nodeID must be positive", ErrHierarchyNodeNotFound)
	}

	if err := h.lockHierarchy(ctx, tx); err != nil {
		return err
	}

	if err := h.adminRowExists(ctx, tx, nodeID); err != nil {
		return err
	}

	var linked int64
	if err := tx.WithContext(ctx).Table(h.closureTable()).Where("descendant_id = ?", nodeID).Count(&linked).Error; err != nil {
		return err
	}
	if linked > 0 {
		return fmt.Errorf("%w: %d", ErrHierarchyAlreadyLinked, nodeID)
	}

	if parentID != nil {
		if *parentID == nodeID {
			return fmt.Errorf("%w: parent equals node %d", ErrHierarchySelfMove, nodeID)
		}
		if err := h.ensureNodeExists(ctx, tx, *parentID); err != nil {
			return err
		}
	}

	var parentArg any
	if parentID != nil {
		parentArg = *parentID
	}
	if err := tx.WithContext(ctx).Exec(
		"UPDATE "+quoteIdentifier(h.adminTable())+" SET `parent_id` = ? WHERE `id` = ?",
		parentArg, nodeID,
	).Error; err != nil {
		return err
	}
	if err := h.adminRowExists(ctx, tx, nodeID); err != nil {
		return err
	}

	if err := tx.WithContext(ctx).Exec(
		"INSERT INTO "+quoteIdentifier(h.closureTable())+" (ancestor_id, descendant_id, depth) VALUES (?, ?, 0)",
		nodeID, nodeID,
	).Error; err != nil {
		return err
	}

	if parentID != nil {
		if err := tx.WithContext(ctx).Exec(
			"INSERT INTO "+quoteIdentifier(h.closureTable())+" (ancestor_id, descendant_id, depth) "+
				"SELECT ancestor_id, ?, depth+1 FROM "+quoteIdentifier(h.closureTable())+" WHERE descendant_id = ?",
			nodeID, *parentID,
		).Error; err != nil {
			return err
		}
	}

	return nil
}

// MoveSubtree validates the move, acquires the same closure-table lock as
// LinkNewNode, checks that admin.parent_id and the closure depth=1 edge are
// consistent, rewrites the ancestor paths of the node subtree, and updates
// admin.parent_id in the same caller-owned transaction. It rejects self-moves,
// descendant-moves, orphan parents and inconsistent closure state.
func (h *AdminHierarchy) MoveSubtree(ctx context.Context, tx *gorm.DB, nodeID int32, newParentID *int32) error {
	if nodeID <= 0 {
		return fmt.Errorf("%w: nodeID must be positive", ErrHierarchyNodeNotFound)
	}

	if err := h.lockHierarchy(ctx, tx); err != nil {
		return err
	}

	if err := h.ensureNodeExists(ctx, tx, nodeID); err != nil {
		return err
	}
	if newParentID != nil {
		if *newParentID == nodeID {
			return ErrHierarchySelfMove
		}
		if err := h.ensureNodeExists(ctx, tx, *newParentID); err != nil {
			return err
		}
		var isDescendant int64
		if err := tx.WithContext(ctx).Table(h.closureTable()).
			Where("ancestor_id = ? AND descendant_id = ?", nodeID, *newParentID).
			Count(&isDescendant).Error; err != nil {
			return err
		}
		if isDescendant > 0 {
			return ErrHierarchyDescendantMove
		}
	}

	var parentCol sql.NullInt32
	result := tx.WithContext(ctx).Raw(
		"SELECT parent_id FROM "+quoteIdentifier(h.adminTable())+" WHERE id = ? FOR UPDATE",
		nodeID,
	).Scan(&parentCol)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: node %d", ErrHierarchyNodeNotFound, nodeID)
	}
	var currentParent *int32
	if parentCol.Valid {
		currentParent = &parentCol.Int32
	}

	var closureParent *int32
	var parents []int32
	if err := tx.WithContext(ctx).Table(h.closureTable()).
		Where("descendant_id = ? AND depth = 1", nodeID).
		Order("ancestor_id").Limit(2).Pluck("ancestor_id", &parents).Error; err != nil {
		return err
	}
	switch len(parents) {
	case 0:
		// root: no direct parent
	case 1:
		parent := parents[0]
		closureParent = &parent
	default:
		return fmt.Errorf("%w: multiple depth=1 parents for node %d", ErrHierarchyIntegrity, nodeID)
	}
	if !int32PtrEqual(currentParent, closureParent) {
		return fmt.Errorf("%w: admin.parent_id=%v, closure parent=%v", ErrHierarchyIntegrity, currentParent, closureParent)
	}

	if int32PtrEqual(currentParent, newParentID) {
		return nil
	}

	var subtree []int32
	if err := tx.WithContext(ctx).Table(h.closureTable()).
		Where("ancestor_id = ?", nodeID).
		Pluck("descendant_id", &subtree).Error; err != nil {
		return err
	}
	if len(subtree) == 0 {
		return fmt.Errorf("%w: node %d", ErrHierarchyNodeNotFound, nodeID)
	}

	if err := tx.WithContext(ctx).Exec(
		"DELETE FROM "+quoteIdentifier(h.closureTable())+
			" WHERE descendant_id IN ? AND ancestor_id NOT IN ?",
		subtree, subtree,
	).Error; err != nil {
		return err
	}

	if newParentID != nil {
		if err := tx.WithContext(ctx).Exec(
			"INSERT INTO "+quoteIdentifier(h.closureTable())+" (ancestor_id, descendant_id, depth) "+
				"SELECT p.ancestor_id, c.descendant_id, p.depth + 1 + c.depth "+
				"FROM "+quoteIdentifier(h.closureTable())+" p "+
				"CROSS JOIN "+quoteIdentifier(h.closureTable())+" c "+
				"WHERE p.descendant_id = ? AND c.ancestor_id = ?",
			*newParentID, nodeID,
		).Error; err != nil {
			return err
		}
	}

	var newParentArg any
	if newParentID != nil {
		newParentArg = *newParentID
	}
	if result := tx.WithContext(ctx).Exec(
		"UPDATE "+quoteIdentifier(h.adminTable())+" SET `parent_id` = ? WHERE `id` = ?",
		newParentArg, nodeID,
	); result.Error != nil {
		return result.Error
	} else if result.RowsAffected != 1 {
		return fmt.Errorf("%w: node %d", ErrHierarchyNodeNotFound, nodeID)
	}

	if err := h.adminRowExists(ctx, tx, nodeID); err != nil {
		return err
	}

	return nil
}
