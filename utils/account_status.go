package utils

// AccountStatusEnabled reports whether an admin/user status is allowed to
// authenticate. Matching is exact: collation-folded or padded values are off.
func AccountStatusEnabled(status string) bool {
	return status == "enable"
}
