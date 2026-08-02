package crud

import adminhandler "buildadmin-go/internal/admin/handler"

type Base = adminhandler.Base
type IDS = adminhandler.IDS
type PartialEditValidator = adminhandler.PartialEditValidator
type Response = adminhandler.Response

var (
	Success            = adminhandler.Success
	SuccessWithMessage = adminhandler.SuccessWithMessage
	FailByErr          = adminhandler.FailByErr
	JsonReturn         = adminhandler.JsonReturn
	CRUDRoutes         = adminhandler.CRUDRoutes
	CRUDCapabilities   = adminhandler.CRUDCapabilities
	NewBase            = adminhandler.NewBase
)
