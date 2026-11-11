package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"buildadmin-go/internal/conf"
	"buildadmin-go/internal/pkg/data_scope"
	cErr "buildadmin-go/internal/pkg/error"

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
	prefix      string
	ownerTables []string
}

func NewAdminHierarchy(config *conf.Configuration) *AdminHierarchy {
	return &AdminHierarchy{
		prefix:      config.Database.Prefix,
		ownerTables: data_scope.AdminOwnerTables(),
	}
}

func (h *AdminHierarchy) adminTable() string   { return h.prefix + "admin" }
func (h *AdminHierarchy) closureTable() string { return h.prefix + "admin_closure" }

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

// lockHierarchy acquires the first admin row as the transaction-scoped
// hierarchy mutex and commit-boundary lock. InnoDB releases it with the
// transaction, detects deadlocks between these row locks, and lets
// innodb_lock_wait_timeout bound waits when no deadlock cycle exists.
func (h *AdminHierarchy) lockHierarchy(tx *gorm.DB) error {
	var anchorID int32
	result := tx.Raw("SELECT id FROM " + quoteIdentifier(h.adminTable()) + " ORDER BY id LIMIT 1 FOR UPDATE").Scan(&anchorID)
	if result.Error != nil {
		// 结果错误只作日志文本：对外唯一可判定的原因必须保持 ErrHierarchyIntegrity，
		// 避免调用方的 errors.Is 误命中底层驱动错误。
		return fmt.Errorf("%w: hierarchy anchor lock failed: %v", ErrHierarchyIntegrity, result.Error) //nolint:errorlint // 见上
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("%w: hierarchy anchor lock failed: anchor row missing", ErrHierarchyIntegrity)
	}
	return nil
}

// LockHierarchy serializes non-hierarchy owner assignments with hierarchy
// mutations. Callers must acquire it before locking user or owner-log rows.
func (h *AdminHierarchy) LockHierarchy(ctx context.Context, tx *gorm.DB) error {
	if err := data_scope.ValidateTablePrefix(h.prefix); err != nil {
		return err
	}
	return h.lockHierarchy(tx)
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

// verifyInScope re-validates, under the already-acquired hierarchy lock, that
// every requested node is visible to the actor. It is the TOCTOU defense for
// scoped moves and deletes.
func (h *AdminHierarchy) verifyInScope(ctx context.Context, tx *gorm.DB, actor data_scope.Actor, enforcer *data_scope.ClosureEnforcer, ids ...int32) error {
	need := make([]int32, 0, len(ids))
	seen := make(map[int32]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return data_scope.ErrScopedAccessDenied
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			need = append(need, id)
		}
	}
	if len(need) == 0 {
		return nil
	}
	if actor.Unrestricted {
		return nil
	}
	if enforcer == nil {
		return data_scope.ErrScopedAccessDenied
	}
	scoped := enforcer.ScopeWithActor(ctx, tx.Table(h.adminTable()).Select("id"), actor, data_scope.OwnerRef{TableAlias: h.adminTable(), Column: "id"})
	if scoped.Error != nil {
		return scoped.Error
	}
	var found []int32
	if err := scoped.Where("id IN ?", need).Pluck("id", &found).Error; err != nil {
		return err
	}
	if len(found) != len(need) {
		return data_scope.ErrScopedAccessDenied
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
	err := h.lockHierarchy(tx)
	if err != nil {
		return err
	}
	return h.linkNewNodeLocked(ctx, tx, nodeID, parentID)
}

// LinkNewNodeWithScope is the scoped variant of LinkNewNode. It re-verifies
// that the chosen parent is still within the actor's scope after the hierarchy
// lock is held, eliminating a TOCTOU window.
func (h *AdminHierarchy) LinkNewNodeWithScope(ctx context.Context, tx *gorm.DB, nodeID int32, parentID *int32, actor data_scope.Actor, enforcer *data_scope.ClosureEnforcer) error {
	if nodeID <= 0 {
		return fmt.Errorf("%w: nodeID must be positive", ErrHierarchyNodeNotFound)
	}
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	if !actor.Unrestricted && parentID == nil {
		return data_scope.ErrScopedAccessDenied
	}
	err := h.lockHierarchy(tx)
	if err != nil {
		return err
	}
	if parentID != nil {
		if err := h.verifyInScope(ctx, tx, actor, enforcer, *parentID); err != nil {
			return err
		}
	}
	return h.linkNewNodeLocked(ctx, tx, nodeID, parentID)
}

func (h *AdminHierarchy) linkNewNodeLocked(ctx context.Context, tx *gorm.DB, nodeID int32, parentID *int32) error {
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
	err := h.lockHierarchy(tx)
	if err != nil {
		return err
	}
	return h.moveSubtreeLocked(ctx, tx, nodeID, newParentID)
}

// MoveSubtreeWithScope is the scoped variant of MoveSubtree. The target node
// and the new parent are re-verified inside the hierarchy lock before any
// mutation occurs.
func (h *AdminHierarchy) MoveSubtreeWithScope(ctx context.Context, tx *gorm.DB, nodeID int32, newParentID *int32, actor data_scope.Actor, enforcer *data_scope.ClosureEnforcer) error {
	if nodeID <= 0 {
		return fmt.Errorf("%w: nodeID must be positive", ErrHierarchyNodeNotFound)
	}
	if data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	if !actor.Unrestricted && newParentID == nil {
		return data_scope.ErrScopedAccessDenied
	}
	err := h.lockHierarchy(tx)
	if err != nil {
		return err
	}
	ids := []int32{nodeID}
	if newParentID != nil {
		ids = append(ids, *newParentID)
	}
	if err := h.verifyInScope(ctx, tx, actor, enforcer, ids...); err != nil {
		return err
	}
	return h.moveSubtreeLocked(ctx, tx, nodeID, newParentID)
}

// ValidateOrMove keeps omitted/null parent edits from using an external
// snapshot. It locks the hierarchy, rechecks the target scope and closure
// integrity, and only performs a move when the caller explicitly changed the
// parent.
func (h *AdminHierarchy) ValidateOrMoveWithScope(ctx context.Context, tx *gorm.DB, nodeID int32, changeParent bool, newParentID *int32, actor data_scope.Actor, enforcer *data_scope.ClosureEnforcer) error {
	if nodeID <= 0 || data_scope.ValidateActor(actor) != nil {
		return data_scope.ErrScopedAccessDenied
	}
	if changeParent && !actor.Unrestricted && newParentID == nil {
		return data_scope.ErrScopedAccessDenied
	}
	err := h.lockHierarchy(tx)
	if err != nil {
		return err
	}
	ids := []int32{nodeID}
	if changeParent && newParentID != nil {
		ids = append(ids, *newParentID)
	}
	if err := h.verifyInScope(ctx, tx, actor, enforcer, ids...); err != nil {
		return err
	}
	if !changeParent {
		_, err := h.validateNodeLocked(ctx, tx, nodeID)
		return err
	}
	return h.moveSubtreeLocked(ctx, tx, nodeID, newParentID)
}

func (h *AdminHierarchy) validateNodeLocked(ctx context.Context, tx *gorm.DB, nodeID int32) (*int32, error) {
	if err := h.ensureNodeExists(ctx, tx, nodeID); err != nil {
		return nil, err
	}
	var parentCol sql.NullInt32
	result := tx.WithContext(ctx).Raw("SELECT parent_id FROM "+quoteIdentifier(h.adminTable())+" WHERE id = ? FOR UPDATE", nodeID).Scan(&parentCol)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, fmt.Errorf("%w: node %d", ErrHierarchyNodeNotFound, nodeID)
	}
	var currentParent *int32
	if parentCol.Valid {
		currentParent = &parentCol.Int32
	}
	var parents []int32
	if err := tx.WithContext(ctx).Table(h.closureTable()).Where("descendant_id = ? AND depth = 1", nodeID).Order("ancestor_id").Limit(2).Pluck("ancestor_id", &parents).Error; err != nil {
		return nil, err
	}
	var closureParent *int32
	switch len(parents) {
	case 1:
		parent := parents[0]
		closureParent = &parent
	case 2:
		return nil, fmt.Errorf("%w: multiple depth=1 parents for node %d", ErrHierarchyIntegrity, nodeID)
	}
	if !int32PtrEqual(currentParent, closureParent) {
		return nil, fmt.Errorf("%w: admin.parent_id=%v, closure parent=%v", ErrHierarchyIntegrity, currentParent, closureParent)
	}
	return currentParent, nil
}

func (h *AdminHierarchy) moveSubtreeLocked(ctx context.Context, tx *gorm.DB, nodeID int32, newParentID *int32) error {
	currentParent, err := h.validateNodeLocked(ctx, tx, nodeID)
	if err != nil {
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

// AdminOwnerTransfer 记录某张归属表在一次删除中被转移的行数。
type AdminOwnerTransfer struct {
	AdminID    int32  `json:"admin_id"`
	NewOwnerID int32  `json:"new_owner_id"`
	Table      string `json:"table"`
	Rows       int64  `json:"rows"`
}

// AdminDeleteResult 汇总一次批量删除，供调用方在事务提交后写审计日志。
type AdminDeleteResult struct {
	Deleted   []int32              `json:"deleted"`
	Transfers []AdminOwnerTransfer `json:"transfers"`
}

type adminDeleteTarget struct {
	id       int32
	parentID *int32
}

// DeleteAdmins 是 DeleteAdminsWithResult 的兼容入口，只返回错误。
func (h *AdminHierarchy) DeleteAdmins(ctx context.Context, tx *gorm.DB, ids []int32, actor data_scope.Actor, enforcer *data_scope.ClosureEnforcer) error {
	_, err := h.DeleteAdminsWithResult(ctx, tx, ids, actor, enforcer)
	return err
}

// DeleteAdminsWithResult deletes a batch of leaf administrators under the
// hierarchy lock. It verifies scope, rejects deletions that would orphan
// existing subordinates, reassigns every registered owner table's rows from
// each target to that target's parent, removes the corresponding closure rows,
// and deletes the admin rows with a RowsAffected check so the operation is
// all-or-nothing. The returned result carries the transfer counts for audit
// logging by the caller after commit.
//
// 关系仍存在于闭包时，父级不可能与被删节点同批（否则父级有下级，早被
// 子级检查拒绝）；这里额外拒绝"父级也在删除集合"的异常闭包，避免转移后
// 立刻再次成为孤儿。parent_id 是框架后加的列，从 PHP 上游升级的库不会回填，
// 因此仅当管理员在已登记的归属表中确实拥有数据时才要求有效的转移目标；
// 顶层（parent_id 为空）且无归属数据的管理员可以直接删除，转移目标还必须
// 存在且处于启用状态。
func (h *AdminHierarchy) DeleteAdminsWithResult(ctx context.Context, tx *gorm.DB, ids []int32, actor data_scope.Actor, enforcer *data_scope.ClosureEnforcer) (*AdminDeleteResult, error) {
	if data_scope.ValidateActor(actor) != nil {
		return nil, data_scope.ErrScopedAccessDenied
	}
	unique := make([]int32, 0, len(ids))
	seen := make(map[int32]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, data_scope.ErrScopedAccessDenied
		}
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return &AdminDeleteResult{}, nil
	}

	err := h.lockHierarchy(tx)
	if err != nil {
		return nil, err
	}
	if err := h.verifyInScope(ctx, tx, actor, enforcer, unique...); err != nil {
		return nil, err
	}

	var childCount int64
	if err := tx.WithContext(ctx).Table(h.closureTable()).
		Where("ancestor_id IN ? AND descendant_id != ancestor_id", unique).
		Count(&childCount).Error; err != nil {
		return nil, err
	}
	if childCount > 0 {
		return nil, cErr.BadRequest("cannot delete administrator with subordinates")
	}

	targets, err := h.deleteTargets(ctx, tx, unique)
	if err != nil {
		return nil, err
	}
	deleting := make(map[int32]struct{}, len(targets))
	for _, target := range targets {
		deleting[target.id] = struct{}{}
	}
	reassign := make(map[int32]int32, len(targets))
	for _, target := range targets {
		owns, err := h.adminOwnsRows(ctx, tx, target.id)
		if err != nil {
			return nil, err
		}
		if !owns {
			continue
		}
		if target.parentID == nil || *target.parentID <= 0 {
			return nil, cErr.BadRequest(fmt.Sprintf("cannot delete top-level administrator %d without a reassignment target", target.id))
		}
		if _, alsoDeleting := deleting[*target.parentID]; alsoDeleting {
			return nil, cErr.BadRequest(fmt.Sprintf("cannot delete administrator %d: its parent %d is also being deleted", target.id, *target.parentID))
		}
		if err := h.ensureNodeExists(ctx, tx, *target.parentID); err != nil {
			return nil, err
		}
		if err := h.ensureReassignmentTargetEnabled(ctx, tx, *target.parentID); err != nil {
			return nil, err
		}
		reassign[target.id] = *target.parentID
	}

	slices.Sort(unique)
	deleteResult := &AdminDeleteResult{Deleted: unique}
	for _, target := range targets {
		newOwnerID, ok := reassign[target.id]
		if !ok {
			continue
		}
		for _, table := range h.ownerTables {
			rows, err := h.transferOwnerRows(ctx, tx, table, target.id, newOwnerID)
			if err != nil {
				return nil, err
			}
			if rows == 0 {
				continue
			}
			deleteResult.Transfers = append(deleteResult.Transfers, AdminOwnerTransfer{
				AdminID:    target.id,
				NewOwnerID: newOwnerID,
				Table:      table,
				Rows:       rows,
			})
		}
	}

	result := tx.WithContext(ctx).Table(h.adminTable()).Where("id IN ?", unique).Delete(nil)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != int64(len(unique)) {
		return nil, cErr.BadRequest("delete failed: rows affected mismatch")
	}
	if err := tx.WithContext(ctx).Exec(
		"DELETE FROM "+quoteIdentifier(h.closureTable())+" WHERE descendant_id IN ?",
		unique,
	).Error; err != nil {
		return nil, err
	}

	return deleteResult, nil
}

// deleteTargets 在层级锁内读取待删管理员的 parent_id。Unrestricted actor 的
// verifyInScope 不检查存在性，因此这里同时承担"目标仍存在"的校验。
func (h *AdminHierarchy) deleteTargets(ctx context.Context, tx *gorm.DB, ids []int32) ([]adminDeleteTarget, error) {
	rows := make([]struct {
		ID       int32
		ParentID sql.NullInt32
	}, 0, len(ids))
	if err := tx.WithContext(ctx).Table(h.adminTable()).
		Select("id, parent_id").Where("id IN ?", ids).Order("id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(ids) {
		return nil, cErr.BadRequest("delete failed: rows affected mismatch")
	}
	targets := make([]adminDeleteTarget, 0, len(rows))
	for _, row := range rows {
		target := adminDeleteTarget{id: row.ID}
		if row.ParentID.Valid {
			parent := row.ParentID.Int32
			target.parentID = &parent
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// adminOwnsRows 报告管理员是否在任一已登记且已物化的归属表中拥有数据。
// 未物化的登记表视为没有数据，与 transferOwnerRows 的缺失表语义一致。
// 只读探测，必须与删除处于同一事务，避免探测与转移之间的并发写入。
func (h *AdminHierarchy) adminOwnsRows(ctx context.Context, tx *gorm.DB, adminID int32) (bool, error) {
	for _, table := range h.ownerTables {
		owned, err := h.adminOwnsRowInTable(ctx, tx, table, adminID)
		if err != nil {
			return false, err
		}
		if owned {
			return true, nil
		}
	}
	return false, nil
}

func (h *AdminHierarchy) adminOwnsRowInTable(ctx context.Context, tx *gorm.DB, table string, adminID int32) (bool, error) {
	if err := data_scope.ValidateIdentifier(table); err != nil {
		return false, fmt.Errorf("admin owner table %q: %w", table, err)
	}
	full := h.prefix + table
	if err := data_scope.ValidateIdentifier(full); err != nil {
		return false, fmt.Errorf("admin owner table %q: %w", full, err)
	}
	if !tx.WithContext(ctx).Migrator().HasTable(full) {
		return false, nil
	}
	var found int
	result := tx.WithContext(ctx).Raw(
		"SELECT 1 FROM "+quoteIdentifier(full)+" WHERE admin_id = ? LIMIT 1",
		adminID,
	).Scan(&found)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// ensureReassignmentTargetEnabled 拒绝把归属数据转挂到不存在或已停用的管理员，
// 否则这些行会落到一个无法登录的账号名下。
func (h *AdminHierarchy) ensureReassignmentTargetEnabled(ctx context.Context, tx *gorm.DB, id int32) error {
	var statuses []string
	if err := tx.WithContext(ctx).Table(h.adminTable()).Where("id = ?", id).Limit(1).Pluck("status", &statuses).Error; err != nil {
		return err
	}
	if len(statuses) == 0 {
		return fmt.Errorf("%w: admin %d", ErrHierarchyNodeNotFound, id)
	}
	if statuses[0] != "enable" {
		return cErr.BadRequest(fmt.Sprintf("reassignment target administrator %d is not enabled", id))
	}
	return nil
}

// transferOwnerRows 把一张归属表中 adminID 的行改挂到 newOwnerID。登记表名
// 必须是合法标识符；尚未物化的登记表直接跳过（没有行需要转移，与 cascade
// 同步的缺失表语义一致）。update_time 仅在列存在时更新，兼容没有该列的
// 历史表。
func (h *AdminHierarchy) transferOwnerRows(ctx context.Context, tx *gorm.DB, table string, adminID, newOwnerID int32) (int64, error) {
	if err := data_scope.ValidateIdentifier(table); err != nil {
		return 0, fmt.Errorf("admin owner table %q: %w", table, err)
	}
	full := h.prefix + table
	if err := data_scope.ValidateIdentifier(full); err != nil {
		return 0, fmt.Errorf("admin owner table %q: %w", full, err)
	}
	if !tx.WithContext(ctx).Migrator().HasTable(full) {
		return 0, nil
	}
	var updateTimeColumns int64
	if err := tx.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?",
		full, "update_time",
	).Scan(&updateTimeColumns).Error; err != nil {
		return 0, err
	}
	updates := map[string]any{"admin_id": newOwnerID}
	if updateTimeColumns > 0 {
		updates["update_time"] = time.Now().Unix()
	}
	result := tx.WithContext(ctx).Table(full).Where("admin_id = ?", adminID).UpdateColumns(updates)
	return result.RowsAffected, result.Error
}
