package utils

// AccountStatusEnabled reports whether an admin/user status is allowed to
// authenticate. PHP only rejects the explicit disabled value; this preserves
// compatibility with legacy 0/1 and other non-disable values.
func AccountStatusEnabled(status string) bool {
	return status != "disable"
}
