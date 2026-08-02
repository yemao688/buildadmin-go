package middleware

import (
	"sort"

	middlewarecore "go-build-admin/internal/middleware"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// These routes are mounted before the protected /admin group and therefore do
// not participate in Authorization. They are intentionally absent from this
// report rather than being treated as missing permission rules.
var authorizationReportBypassRoutes = map[string]struct{}{
	"GET /admin/Index/login":         {},
	"POST /admin/Index/login":        {},
	"GET /admin/ajax/buildSuffixSvg": {},
	"GET /admin/ajax/terminal":       {},
}

// ReportUnprotectedRoutes reports admin routes that have neither an admin_rule
// nor an explicit PermissionExempt declaration. It is diagnostic only: rule
// lookup failures are logged and never returned to the startup caller.
func (m *Authorization) ReportUnprotectedRoutes(routes gin.RoutesInfo) {
	if m == nil || m.authM == nil {
		return
	}

	if err := m.authM.DatabaseAvailable(); err != nil {
		if m.log != nil {
			m.log.Warn("admin route protection report skipped: database unavailable", zap.Error(err))
		}
		return
	}

	ruleNames, err := m.authM.GetAllRuleNames()
	if err != nil {
		if m.log != nil {
			m.log.Warn("admin route protection report unavailable", zap.Error(err))
		}
		return
	}
	rules := make(map[string]struct{}, len(ruleNames))
	for _, name := range ruleNames {
		rules[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
	}

	missing := collectUnprotectedRoutes(routes, rules)
	if len(missing) == 0 || m.log == nil {
		return
	}
	m.log.Warn("admin routes are unregistered and not exempt",
		zap.Int("count", len(missing)),
		zap.Strings("routes", missing),
	)
}

func collectUnprotectedRoutes(routes gin.RoutesInfo, ruleNames map[string]struct{}) []string {
	missing := make([]string, 0)
	for _, route := range routes {
		if _, bypass := authorizationReportBypassRoutes[route.Method+" "+route.Path]; bypass {
			continue
		}
		controller, action, ok := middlewarecore.NormalizeRouteAction(route.Path)
		if !ok || IsPermissionExempt(controller, action) {
			continue
		}
		if _, registered := ruleNames[controller+"/"+action]; registered {
			continue
		}
		missing = append(missing, route.Method+" "+route.Path)
	}
	sort.Strings(missing)
	return missing
}
