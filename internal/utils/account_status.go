package utils

// AccountStatusEnabled reports whether an admin/user status is allowed to
// authenticate.
func AccountStatusEnabled(status string) bool {
	return status == "enable"
}
