package middleware

import (
	"strings"
	"sync"
)

// noNeedLoginExemptions 登记"无需登录"的 controller/action 集合，结构
// 与 permissionExemptions 完全一致（map[controller]map[action]，支持 * 通配）。
// 与 permissionExemptions 分开存储：noNeedLogin 跳过整个鉴权链（登录+权限），
// permission exempt 仅跳过规则校验（仍需登录）。
var noNeedLoginExemptions = struct {
	sync.RWMutex
	actions map[string]map[string]struct{}
}{
	actions: make(map[string]map[string]struct{}),
}

// RegisterNoNeedLogin adds actions that do not require the login middleware.
// Controllers and actions are normalized to the same lowercase route form
// used by NormalizeRouteAction. "*" exempts the whole controller.
func RegisterNoNeedLogin(controller string, actions ...string) {
	controller = normalizePermissionController(controller)
	if controller == "" {
		return
	}

	noNeedLoginExemptions.Lock()
	defer noNeedLoginExemptions.Unlock()
	set := noNeedLoginExemptions.actions[controller]
	if set == nil {
		set = make(map[string]struct{})
		noNeedLoginExemptions.actions[controller] = set
	}
	for _, action := range actions {
		action = strings.ToLower(strings.TrimSpace(action))
		if action != "" {
			set[action] = struct{}{}
		}
	}
}

// UnregisterNoNeedLogin removes actions previously registered. It is
// primarily useful to keep isolated tests from sharing global state.
func UnregisterNoNeedLogin(controller string, actions ...string) {
	controller = normalizePermissionController(controller)
	if controller == "" {
		return
	}

	noNeedLoginExemptions.Lock()
	defer noNeedLoginExemptions.Unlock()
	set := noNeedLoginExemptions.actions[controller]
	if set == nil {
		return
	}
	for _, action := range actions {
		delete(set, strings.ToLower(strings.TrimSpace(action)))
	}
	if len(set) == 0 {
		delete(noNeedLoginExemptions.actions, controller)
	}
}

// IsNoNeedLogin reports whether an action is login-exempt or its controller
// has a wildcard exemption.
func IsNoNeedLogin(controller, action string) bool {
	controller = normalizePermissionController(controller)
	action = strings.ToLower(strings.TrimSpace(action))
	if controller == "" || action == "" {
		return false
	}

	noNeedLoginExemptions.RLock()
	defer noNeedLoginExemptions.RUnlock()
	set := noNeedLoginExemptions.actions[controller]
	if set == nil {
		return false
	}
	_, wildcard := set["*"]
	_, exact := set[action]
	return wildcard || exact
}

// RegisterHandlerExemptions collects the exemption declarations from a
// handler and registers them: NoNeedLoginer feeds the noNeedLogin registry,
// NoNeedPermissioner feeds the permission exemption registry. Controllers
// without the corresponding interface simply contribute nothing.
//
// The controller name is explicit because the route prefix cannot always be
// derived from the handler type (e.g. alioss/callback lives on AjaxHandler).
func RegisterHandlerExemptions(controller string, h any) {
	if n, ok := h.(interface{ NoNeedLoginActions() []string }); ok {
		RegisterNoNeedLogin(controller, n.NoNeedLoginActions()...)
	}
	if p, ok := h.(interface{ NoNeedPermissionActions() []string }); ok {
		RegisterPermissionExempt(controller, p.NoNeedPermissionActions()...)
	}
}
