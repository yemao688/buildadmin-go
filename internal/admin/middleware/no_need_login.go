package middleware

// noNeedLoginExemptions 登记"无需登录"的 controller/action 集合，
// 与 permissionExemptions 分开存储，语义见 actionExemptionRegistry。
var noNeedLoginExemptions = newActionExemptionRegistry()

// RegisterNoNeedLogin adds actions that do not require the login middleware.
// Controllers and actions are normalized to the same lowercase route form
// used by NormalizeRouteAction. "*" exempts the whole controller.
func RegisterNoNeedLogin(controller string, actions ...string) {
	noNeedLoginExemptions.register(controller, actions...)
}

// UnregisterNoNeedLogin removes actions previously registered. It is
// primarily useful to keep isolated tests from sharing global state.
func UnregisterNoNeedLogin(controller string, actions ...string) {
	noNeedLoginExemptions.unregister(controller, actions...)
}

// IsNoNeedLogin reports whether an action is login-exempt or its controller
// has a wildcard exemption.
func IsNoNeedLogin(controller, action string) bool {
	return noNeedLoginExemptions.isExempt(controller, action)
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
