package util

import "strings"

// NormalizeControllerAs 将点形控制器名归一化为小写斜杠形式
//（security.DataRecycle → security/datarecycle）。
func NormalizeControllerAs(controller string) string {
	return strings.ToLower(strings.ReplaceAll(controller, ".", "/"))
}