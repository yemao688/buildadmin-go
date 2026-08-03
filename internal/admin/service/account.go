package service

import (
	cErr "buildadmin-go/internal/pkg/error"
)

// ValidateAccountStatusValue enforces the account-status vocabulary shared by
// the admin and user management flows. Account status only accepts
// enable/disable; other status fields keep the legacy 0/1 protocol.
func ValidateAccountStatusValue(value any) error {
	status, ok := value.(string)
	if !ok || (status != "enable" && status != "disable") {
		return cErr.BadRequest("status must be enable or disable")
	}
	return nil
}
