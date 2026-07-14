package data_scope

import (
	"context"

	"gorm.io/gorm"
)

// StubHierarchyWriter is a fail-closed HierarchyWriter scaffold. It accepts
// the transaction owned by the caller and returns ErrNotImplemented. The
// caller must still drive commit/rollback; this type never does.
type StubHierarchyWriter struct{}

// NewStubHierarchyWriter returns a writer that safely compiles but refuses
// every mutation.
func NewStubHierarchyWriter() *StubHierarchyWriter {
	return &StubHierarchyWriter{}
}

// LinkNewNode is intentionally not implemented in the contract lane.
func (StubHierarchyWriter) LinkNewNode(ctx context.Context, tx *gorm.DB, nodeID int32, parentID *int32) error {
	_ = ctx
	_ = tx
	_ = nodeID
	_ = parentID
	return ErrNotImplemented
}

// MoveSubtree is intentionally not implemented in the contract lane.
func (StubHierarchyWriter) MoveSubtree(ctx context.Context, tx *gorm.DB, nodeID int32, newParentID *int32) error {
	_ = ctx
	_ = tx
	_ = nodeID
	_ = newParentID
	return ErrNotImplemented
}
