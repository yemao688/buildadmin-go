// Package util 是跨层通用工具（路径/时间/转换/数组/加密等），不得依赖业务渠道。
package util

// AccountStatusEnabled reports whether an admin/user status is allowed to
// authenticate.
func AccountStatusEnabled(status string) bool {
	return status == "enable"
}
