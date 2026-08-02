package handler

import "strings"

func normalizeControllerAs(controller string) string {
	return strings.ToLower(strings.ReplaceAll(controller, ".", "/"))
}
