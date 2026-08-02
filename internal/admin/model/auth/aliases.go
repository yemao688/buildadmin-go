package auth

import (
	adminmodel "go-build-admin/internal/admin/model"
	model "go-build-admin/internal/model"
	persistence "go-build-admin/internal/pkg/persistence"
)

type BaseModel = persistence.BaseModel
type AdminHierarchy = adminmodel.AdminHierarchy
type AdminGroupAccess = model.AdminGroupAccess
type AdminClosure = model.AdminClosure

type Admin = model.Admin
type AdminSummary = model.AdminSummary
type AdminGroup = model.AdminGroup
type AdminRule = model.AdminRule
type AdminLog = model.AdminLog

var (
	NewBaseModel      = persistence.NewBaseModel
	QueryBuilder      = adminmodel.QueryBuilder
	IsSuperAdmin      = adminmodel.IsSuperAdmin
	NewAdminHierarchy = adminmodel.NewAdminHierarchy

	ErrHierarchyOrphanParent   = adminmodel.ErrHierarchyOrphanParent
	ErrHierarchySelfMove       = adminmodel.ErrHierarchySelfMove
	ErrHierarchyDescendantMove = adminmodel.ErrHierarchyDescendantMove
	ErrHierarchyIntegrity      = adminmodel.ErrHierarchyIntegrity
)
