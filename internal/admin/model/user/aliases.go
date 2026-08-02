package user

import model "go-build-admin/internal/admin/model"

type BaseModel = model.BaseModel
type AdminHierarchy = model.AdminHierarchy
type AdminClosure = model.AdminClosure
type AdminGroupAccess = model.AdminGroupAccess

var (
	NewBaseModel      = model.NewBaseModel
	QueryBuilder      = model.QueryBuilder
	NewAdminHierarchy = model.NewAdminHierarchy
)
