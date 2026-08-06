package middleware

import (
	"strings"
	"sync"
)

// actionExemptionRegistry 是 controller→action 豁免集合的并发安全注册表，
// 支持 "*" 通配整控制器。noNeedLogin 与 permissionExempt 各持一份实例，
// 二者语义必须保持隔离：noNeedLogin 跳过整个鉴权链（登录+权限），
// permission exempt 仅跳过规则校验（仍需登录）。
type actionExemptionRegistry struct {
	mu      sync.RWMutex
	actions map[string]map[string]struct{}
}

func newActionExemptionRegistry() *actionExemptionRegistry {
	return &actionExemptionRegistry{
		actions: make(map[string]map[string]struct{}),
	}
}

// register adds actions for a controller. Controllers and actions are
// normalized to the same lowercase route form used by authorization.go.
// "*" exempts the whole controller.
func (r *actionExemptionRegistry) register(controller string, actions ...string) {
	controller = normalizePermissionController(controller)
	if controller == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	set := r.actions[controller]
	if set == nil {
		set = make(map[string]struct{})
		r.actions[controller] = set
	}
	for _, action := range actions {
		action = strings.ToLower(strings.TrimSpace(action))
		if action != "" {
			set[action] = struct{}{}
		}
	}
}

// unregister removes actions previously registered. It is primarily useful
// to keep isolated tests from sharing global registration state.
func (r *actionExemptionRegistry) unregister(controller string, actions ...string) {
	controller = normalizePermissionController(controller)
	if controller == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	set := r.actions[controller]
	if set == nil {
		return
	}
	for _, action := range actions {
		delete(set, strings.ToLower(strings.TrimSpace(action)))
	}
	if len(set) == 0 {
		delete(r.actions, controller)
	}
}

// isExempt reports whether an action is explicitly exempt or its controller
// has a wildcard exemption.
func (r *actionExemptionRegistry) isExempt(controller, action string) bool {
	controller = normalizePermissionController(controller)
	action = strings.ToLower(strings.TrimSpace(action))
	if controller == "" || action == "" {
		return false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	set := r.actions[controller]
	if set == nil {
		return false
	}
	_, wildcard := set["*"]
	_, exact := set[action]
	return wildcard || exact
}

// permissionExemptions 登记"免规则校验"（仍需登录）的 controller/action 集合，
// 与 noNeedLoginExemptions 分开存储，语义见 actionExemptionRegistry。
var permissionExemptions = newActionExemptionRegistry()

// RegisterPermissionExempt adds actions that do not require an admin_rule.
// Controllers and actions are normalized to the same lowercase route form
// used by authorization.go.
func RegisterPermissionExempt(controller string, actions ...string) {
	permissionExemptions.register(controller, actions...)
}

// UnregisterPermissionExempt removes actions previously registered by a
// registrar. It is primarily useful to keep isolated tests from sharing
// global registration state.
func UnregisterPermissionExempt(controller string, actions ...string) {
	permissionExemptions.unregister(controller, actions...)
}

// IsPermissionExempt reports whether an action is explicitly exempt or its
// controller has a wildcard exemption.
func IsPermissionExempt(controller, action string) bool {
	return permissionExemptions.isExempt(controller, action)
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
