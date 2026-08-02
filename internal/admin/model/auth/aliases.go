package auth

import model "go-build-admin/internal/admin/model"

type BaseModel = model.BaseModel
type AdminHierarchy = model.AdminHierarchy
type AdminGroupAccess = model.AdminGroupAccess
type AdminClosure = model.AdminClosure

var (
	NewBaseModel      = model.NewBaseModel
	QueryBuilder      = model.QueryBuilder
	IsSuperAdmin      = model.IsSuperAdmin
	NewAdminHierarchy = model.NewAdminHierarchy

	ErrHierarchyOrphanParent   = model.ErrHierarchyOrphanParent
	ErrHierarchySelfMove       = model.ErrHierarchySelfMove
	ErrHierarchyDescendantMove = model.ErrHierarchyDescendantMove
	ErrHierarchyIntegrity      = model.ErrHierarchyIntegrity
)
