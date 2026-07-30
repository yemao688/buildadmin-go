package auth

import adminhandler "go-build-admin/app/admin/handler"

type Base = adminhandler.Base
type IDS = adminhandler.IDS
type PartialEditValidator = adminhandler.PartialEditValidator

var (
	Success            = adminhandler.Success
	SuccessWithMessage = adminhandler.SuccessWithMessage
	FailByErr          = adminhandler.FailByErr
	CRUDRoutes         = adminhandler.CRUDRoutes
	CRUDCapabilities   = adminhandler.CRUDCapabilities
	NewBase            = adminhandler.NewBase
)
