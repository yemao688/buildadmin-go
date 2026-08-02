package user

import (
	adminmodel "go-build-admin/internal/admin/model"
	model "go-build-admin/internal/model"
	persistence "go-build-admin/internal/pkg/persistence"
)

type BaseModel = persistence.BaseModel
type AdminHierarchy = adminmodel.AdminHierarchy
type AdminClosure = model.AdminClosure
type AdminGroupAccess = model.AdminGroupAccess

type User = model.User
type MoneyLog = model.MoneyLog

var (
	NewBaseModel      = persistence.NewBaseModel
	QueryBuilder      = adminmodel.QueryBuilder
	NewAdminHierarchy = adminmodel.NewAdminHierarchy
)
