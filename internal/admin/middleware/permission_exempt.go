package middleware

import (
	"strings"
	"sync"
)

var permissionExemptions = struct {
	sync.RWMutex
	actions map[string]map[string]struct{}
}{
	actions: make(map[string]map[string]struct{}),
}

// RegisterPermissionExempt adds actions that do not require an admin_rule.
// Controllers and actions are normalized to the same lowercase route form
// used by authorization.go.
func RegisterPermissionExempt(controller string, actions ...string) {
	controller = normalizePermissionController(controller)
	if controller == "" {
		return
	}

	permissionExemptions.Lock()
	defer permissionExemptions.Unlock()
	set := permissionExemptions.actions[controller]
	if set == nil {
		set = make(map[string]struct{})
		permissionExemptions.actions[controller] = set
	}
	for _, action := range actions {
		action = strings.ToLower(strings.TrimSpace(action))
		if action != "" {
			set[action] = struct{}{}
		}
	}
}

// UnregisterPermissionExempt removes actions previously registered by a
// registrar. It is primarily useful to keep isolated tests from sharing
// global registration state.
func UnregisterPermissionExempt(controller string, actions ...string) {
	controller = normalizePermissionController(controller)
	if controller == "" {
		return
	}

	permissionExemptions.Lock()
	defer permissionExemptions.Unlock()
	set := permissionExemptions.actions[controller]
	if set == nil {
		return
	}
	for _, action := range actions {
		delete(set, strings.ToLower(strings.TrimSpace(action)))
	}
	if len(set) == 0 {
		delete(permissionExemptions.actions, controller)
	}
}

// IsPermissionExempt reports whether an action is explicitly exempt or its
// controller has a wildcard exemption.
func IsPermissionExempt(controller, action string) bool {
	controller = normalizePermissionController(controller)
	action = strings.ToLower(strings.TrimSpace(action))
	if controller == "" || action == "" {
		return false
	}

	permissionExemptions.RLock()
	defer permissionExemptions.RUnlock()
	set := permissionExemptions.actions[controller]
	if set == nil {
		return false
	}
	_, wildcard := set["*"]
	_, exact := set[action]
	return wildcard || exact
}

// PermissionExempt is the short form used by authorization checks.
func PermissionExempt(controller, action string) bool {
	return IsPermissionExempt(controller, action)
}

func normalizePermissionController(controller string) string {
	controller = strings.TrimSpace(controller)
	controller = strings.ReplaceAll(controller, ".", "/")
	return strings.ToLower(strings.Trim(controller, "/"))
}
